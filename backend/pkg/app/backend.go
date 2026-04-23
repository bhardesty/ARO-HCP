// Copyright 2026 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	_ "k8s.io/component-base/metrics/prometheus/clientgo"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	k8sutilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	utilsclock "k8s.io/utils/clock"

	azureclient "github.com/Azure/ARO-HCP/backend/pkg/azure/client"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/billingcontrollers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/clusterpropertiescontroller"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/controllerutils"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/datadumpcontrollers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/externalauthpropertiescontroller"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/metricscontrollers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/mismatchcontrollers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/nodepoolpropertiescontroller"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/operationcontrollers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/upgradecontrollers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/validationcontrollers"
	"github.com/Azure/ARO-HCP/backend/pkg/controllers/validationcontrollers/validations"
	"github.com/Azure/ARO-HCP/backend/pkg/informers"
	"github.com/Azure/ARO-HCP/backend/pkg/maestro"
	internalazure "github.com/Azure/ARO-HCP/internal/azure"
	"github.com/Azure/ARO-HCP/internal/database"
	"github.com/Azure/ARO-HCP/internal/ocm"
	"github.com/Azure/ARO-HCP/internal/utils"
)

type Backend struct {
	options *BackendOptions
}

type BackendOptions struct {
	AppShortDescriptionName            string
	AppVersion                         string
	AzureLocation                      string
	LeaderElectionLock                 resourcelock.Interface
	CosmosDBClient                     database.DBClient
	ClustersServiceClient              ocm.ClusterServiceClientSpec
	MetricsRegisterer                  prometheus.Registerer
	MetricsGatherer                    prometheus.Gatherer
	MetricsServerListenAddress         string
	MetricsServerListener              net.Listener
	HealthzServerListenAddress         string
	TracerProviderShutdownFunc         func(context.Context) error
	MaestroSourceEnvironmentIdentifier string
	FPAClientBuilder                   azureclient.FirstPartyApplicationClientBuilder
	BackendIdentityAzureClients        *azureclient.BackendIdentityAzureClients
	ExitOnPanic                        bool
	FPAMIDataplaneClientBuilder        azureclient.FPAMIDataplaneClientBuilder
	SMIClientBuilder                   azureclient.ServiceManagedIdentityClientBuilder
	CheckAccessV2ClientBuilder         azureclient.CheckAccessV2ClientBuilder
	ClusterScopedIdentitiesConfig      *internalazure.ClusterScopedIdentitiesConfig
}

func (o *BackendOptions) RunBackend(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(fmt.Errorf("function returned"))

	backend, err := NewBackend(o)
	if err != nil {
		return utils.TrackError(fmt.Errorf("failed to construct backend: %w", err))
	}
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- backend.Run(ctx)
		cancel(fmt.Errorf("backend exited"))
	}()

	<-ctx.Done()
	logger.Info("context closed")

	logger.Info("waiting for backend run to finish")
	runErr := <-runErrCh
	if runErr != nil {
		return utils.TrackError(fmt.Errorf("failed to run backend: %w", runErr))
	}

	return nil
}

func NewBackend(options *BackendOptions) (*Backend, error) {
	if options == nil {
		return nil, errors.New("BackendOptions must not be nil")
	}
	if err := options.validate(); err != nil {
		return nil, err
	}
	return &Backend{
		options: options,
	}, nil
}

// validate checks BackendOptions for invariants that must hold before Run.
// Any failure here is a programmer error in the calling code (flag wiring or
// test setup), not a user-facing condition — we fail fast before any goroutine,
// tracer, or leader-election resource is allocated.
func (o *BackendOptions) validate() error {
	// Registerer and Gatherer must both be explicitly wired by the caller.
	// The production path sets them in cmd/root.go; tests must inject their
	// own. A single half-configured field would silently expose metrics from
	// one registry while populating another, so we refuse to start.
	if o.MetricsRegisterer == nil || o.MetricsGatherer == nil {
		return fmt.Errorf("BackendOptions.MetricsRegisterer and BackendOptions.MetricsGatherer must both be set (MetricsRegisterer set=%t, MetricsGatherer set=%t)",
			o.MetricsRegisterer != nil, o.MetricsGatherer != nil)
	}
	return nil
}

func (b *Backend) Run(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)
	logger.Info("Running backend")

	logger.Info(fmt.Sprintf(
		"%s (%s) started in %s",
		b.options.AppShortDescriptionName,
		b.options.AppVersion,
		b.options.AzureLocation))

	var healthzServer *http.Server
	var metricsServer *http.Server

	ctx, cancel := context.WithCancelCause(ctx)
	defer func() {
		cancel(fmt.Errorf("run returned"))

		// always attempt a graceful shutdown, a double ctrl+c exits the process
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 31*time.Second)
		defer shutdownCancel()
		_ = b.shutdownHTTPServer(shutdownCtx, metricsServer, "metrics server")
		_ = b.shutdownHTTPServer(shutdownCtx, healthzServer, "healthz server")

		logger.Info("shutting down tracer provider")
		err := b.options.TracerProviderShutdownFunc(shutdownCtx)
		if err != nil {
			logger.Error(err, "failed to shut down tracer provider")
		} else {
			logger.Info("tracer provider shut down completed")
		}
	}()

	// We set k8s.io/apimachinery/pkg/util/runtime.ReallyCrash to the value of the ExitOnPanic option to
	// control the behavior of k8s.io/apimachinery/pkg/util/runtime.HandleCrash* methods
	k8sutilruntime.ReallyCrash = b.options.ExitOnPanic

	// Create HealthzAdaptor for leader election
	electionChecker := leaderelection.NewLeaderHealthzAdaptor(time.Second * 20)

	// Create channels and wait group for goroutines.
	// The size of the channel is the maximum number of goroutines that are to be
	// executed.
	// The wait group is used to wait for all goroutines to complete. The sync.WaitGroup counter
	// is incremented for each goroutine that is to be executed according to the configuration.
	errCh := make(chan error, 3)
	wg := sync.WaitGroup{}

	healthzServer = b.serveHealthz(ctx, electionChecker, &wg, errCh)

	var err error
	metricsServer, err = b.serveMetrics(ctx, &wg, errCh)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.runBackendControllersUnderLeaderElection(ctx, electionChecker)
		// When leader election exits (e.g. lost lease), cancel so Run() unblocks and performs shutdown.
		cancel(fmt.Errorf("backend controllers leader election exited"))
		errCh <- err
	}()

	<-ctx.Done()

	// always attempt a graceful shutdown, a double ctrl+c exits the process
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 31*time.Second)
	defer shutdownCancel()
	_ = b.shutdownHTTPServer(shutdownCtx, metricsServer, "metrics server")
	_ = b.shutdownHTTPServer(shutdownCtx, healthzServer, "healthz server")

	wg.Wait()
	close(errCh)
	errs := []error{}
	for err := range errCh {
		if err != nil {
			logger.Info("go func completed", "message", err.Error())
		}
		if !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, err)
		}
	}

	logger.Info(fmt.Sprintf("%s (%s) stopped", b.options.AppShortDescriptionName, b.options.AppVersion))

	return errors.Join(errs...)
}

// serveHealthz starts the /healthz HTTP server on its own ServeMux, in a dedicated
// goroutine, if the option is configured. It returns the *http.Server so the caller
// can shut it down. Returns nil when HealthzServerListenAddress is empty.
func (b *Backend) serveHealthz(ctx context.Context, electionChecker *leaderelection.HealthzAdaptor, wg *sync.WaitGroup, errCh chan<- error) *http.Server {
	if b.options.HealthzServerListenAddress == "" {
		return nil
	}
	logger := utils.LoggerFromContext(ctx)

	backendHealthGauge := promauto.With(b.options.MetricsRegisterer).NewGauge(prometheus.GaugeOpts{Name: "backend_health", Help: "backend_health is 1 when healthy"})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := electionChecker.Check(r); err != nil {
			logger.Error(err, "Readiness probe failed")
			http.Error(w, "lease not renewed", http.StatusServiceUnavailable)
			backendHealthGauge.Set(0.0)
			return
		}
		w.WriteHeader(http.StatusOK)
		backendHealthGauge.Set(1.0)
	})

	server := &http.Server{Addr: b.options.HealthzServerListenAddress, Handler: mux}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(fmt.Sprintf("Healthz server listening on %s", b.options.HealthzServerListenAddress))
		errCh <- server.ListenAndServe()
	}()

	return server
}

// serveMetrics starts the /metrics HTTP server on its own ServeMux, in a dedicated
// goroutine, if the option is configured. When MetricsServerListener is not
// pre-supplied, a net.Listener is created from MetricsServerListenAddress here
// so the socket only exists while Run is executing (avoiding leaked listeners
// if Run is never called). It returns the *http.Server so the caller can shut
// it down. Returns (nil, nil) when neither listener nor address is configured.
func (b *Backend) serveMetrics(ctx context.Context, wg *sync.WaitGroup, errCh chan<- error) (_ *http.Server, retErr error) {
	listener := b.options.MetricsServerListener
	ownListener := false
	if listener == nil {
		if b.options.MetricsServerListenAddress == "" {
			return nil, nil
		}
		l, err := net.Listen("tcp", b.options.MetricsServerListenAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to listen on %s: %w", b.options.MetricsServerListenAddress, err)
		}
		listener = l
		ownListener = true
	}
	// If we created the listener and something below fails before we hand
	// it off to the serving goroutine, close it here so the socket does
	// not leak.
	handedOff := false
	defer func() {
		if ownListener && !handedOff && retErr != nil {
			_ = listener.Close()
		}
	}()

	logger := utils.LoggerFromContext(ctx)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.InstrumentMetricHandler(
		b.options.MetricsRegisterer,
		promhttp.HandlerFor(
			prometheus.Gatherers{b.options.MetricsGatherer},
			promhttp.HandlerOpts{},
		),
	))

	addr := listener.Addr().String()
	server := &http.Server{Addr: addr, Handler: mux}

	wg.Add(1)
	handedOff = true
	go func() {
		defer wg.Done()
		logger.Info(fmt.Sprintf("metrics server listening on %s", addr))
		errCh <- server.Serve(listener)
	}()

	return server, nil
}

// shutdownHTTPServer shuts down an HTTP server, logging its outcome and returning
// an error if the shutdown failed. If the provided server is nil, no action is taken.
// name is a descriptive name for the server, used in the logging.
func (b *Backend) shutdownHTTPServer(ctx context.Context, server *http.Server, name string) error {
	if server == nil {
		return nil
	}
	logger := utils.LoggerFromContext(ctx)

	logger.Info(fmt.Sprintf("shutting down %s", name))
	err := server.Shutdown(ctx)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to shut down %s", name))
	} else {
		logger.Info(fmt.Sprintf("%s shut down completed", name))
	}

	return err
}

// runBackendControllersUnderLeaderElection runs the backen controllers under
// a leader election loop.
func (b *Backend) runBackendControllersUnderLeaderElection(ctx context.Context, electionChecker *leaderelection.HealthzAdaptor) error {
	backendInformers := informers.NewBackendInformers(ctx, b.options.CosmosDBClient.GlobalListers())

	_, subscriptionLister := backendInformers.Subscriptions()
	activeOperationInformer, activeOperationLister := backendInformers.ActiveOperations()

	operationPhaseHandler := metricscontrollers.NewOperationPhaseMetricsHandler(b.options.MetricsRegisterer)
	operationPhaseMetricsController := metricscontrollers.NewController(
		"OperationPhaseMetrics", backendInformers.AllOperations(), operationPhaseHandler)

	clusterInformer, clusterLister := backendInformers.Clusters()
	clusterHandler := metricscontrollers.NewClusterMetricsHandler(b.options.MetricsRegisterer)
	clusterMetricsController := metricscontrollers.NewController(
		"ClusterMetrics", clusterInformer, clusterHandler)

	_, billingLister := backendInformers.BillingDocs()

	nodePoolInformer, _ := backendInformers.NodePools()
	nodePoolHandler := metricscontrollers.NewNodePoolMetricsHandler(b.options.MetricsRegisterer)
	nodePoolMetricsController := metricscontrollers.NewController(
		"NodePoolMetrics", nodePoolInformer, nodePoolHandler)

	externalAuthInformer, _ := backendInformers.ExternalAuths()
	externalAuthHandler := metricscontrollers.NewExternalAuthMetricsHandler(b.options.MetricsRegisterer)
	externalAuthMetricsController := metricscontrollers.NewController(
		"ExternalAuthMetrics", externalAuthInformer, externalAuthHandler)

	maestroClientBuilder := maestro.NewMaestroClientBuilder()

	subscriptionNonClusterDataDumpController := datadumpcontrollers.NewSubscriptionNonClusterDataDumpController(b.options.CosmosDBClient, activeOperationLister, backendInformers)
	clusterRecursiveDataDumpController := datadumpcontrollers.NewClusterRecursiveDataDumpController(b.options.CosmosDBClient, activeOperationLister, backendInformers)
	csStateDumpController := datadumpcontrollers.NewCSStateDumpController(b.options.CosmosDBClient, activeOperationLister, backendInformers, b.options.ClustersServiceClient)
	billingDumpController := datadumpcontrollers.NewBillingDumpController(b.options.CosmosDBClient, activeOperationLister, backendInformers)
	doNothingController := controllers.NewDoNothingExampleController(b.options.CosmosDBClient, subscriptionLister)
	operationClusterCreateController := operationcontrollers.NewOperationClusterCreateController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationClusterUpdateController := operationcontrollers.NewOperationClusterUpdateController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationClusterDeleteController := operationcontrollers.NewOperationClusterDeleteController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationNodePoolCreateController := operationcontrollers.NewOperationNodePoolCreateController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationNodePoolUpdateController := operationcontrollers.NewOperationNodePoolUpdateController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationNodePoolDeleteController := operationcontrollers.NewOperationNodePoolDeleteController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationExternalAuthCreateController := operationcontrollers.NewOperationExternalAuthCreateController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationExternalAuthUpdateController := operationcontrollers.NewOperationExternalAuthUpdateController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationExternalAuthDeleteController := operationcontrollers.NewOperationExternalAuthDeleteController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationRequestCredentialController := operationcontrollers.NewOperationRequestCredentialController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	operationRevokeCredentialsController := operationcontrollers.NewOperationRevokeCredentialsController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		http.DefaultClient,
		activeOperationInformer,
	)
	clusterServiceMatchingClusterController := mismatchcontrollers.NewClusterServiceClusterMatchingController(b.options.CosmosDBClient, subscriptionLister, b.options.ClustersServiceClient)
	cosmosMatchingNodePoolController := mismatchcontrollers.NewCosmosNodePoolMatchingController(b.options.CosmosDBClient, b.options.ClustersServiceClient, backendInformers)
	cosmosMatchingExternalAuthController := mismatchcontrollers.NewCosmosExternalAuthMatchingController(b.options.CosmosDBClient, b.options.ClustersServiceClient, backendInformers)
	cosmosMatchingClusterController := mismatchcontrollers.NewCosmosClusterMatchingController(utilsclock.RealClock{}, b.options.CosmosDBClient, b.options.ClustersServiceClient, backendInformers)
	alwaysSuccessClusterValidationController := validationcontrollers.NewClusterValidationController(
		validations.NewAlwaysSuccessValidation(),
		activeOperationLister,
		b.options.CosmosDBClient,
		backendInformers,
	)
	deleteOrphanedCosmosResourcesController := mismatchcontrollers.NewDeleteOrphanedCosmosResourcesController(b.options.CosmosDBClient, subscriptionLister)
	backfillClusterUIDController := controllerutils.NewClusterWatchingController(
		"BackfillClusterUID", b.options.CosmosDBClient, backendInformers, 60*time.Minute,
		mismatchcontrollers.NewBackfillClusterUIDController(utilsclock.RealClock{}, b.options.CosmosDBClient, clusterLister))
	orphanedBillingCleanupController := billingcontrollers.NewOrphanedBillingCleanupController(utilsclock.RealClock{}, b.options.CosmosDBClient, clusterLister, billingLister)
	createBillingDocController := controllerutils.NewClusterWatchingController(
		"CreateBillingDoc", b.options.CosmosDBClient, backendInformers, 60*time.Second,
		billingcontrollers.NewCreateBillingDocController(utilsclock.RealClock{}, b.options.AzureLocation, b.options.CosmosDBClient, clusterLister, billingLister))
	controlPlaneActiveVersionController := upgradecontrollers.NewControlPlaneActiveVersionController(
		b.options.CosmosDBClient,
		activeOperationLister,
		backendInformers,
	)
	controlPlaneDesiredVersionController := upgradecontrollers.NewControlPlaneDesiredVersionController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
		subscriptionLister,
	)
	triggerControlPlaneUpgradeController := upgradecontrollers.NewTriggerControlPlaneUpgradeController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)
	clusterPropertiesSyncController := clusterpropertiescontroller.NewClusterPropertiesSyncController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)
	clusterServiceMigrationController := clusterpropertiescontroller.NewClusterCustomerPropertiesMigrationController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)
	identityMigrationController := clusterpropertiescontroller.NewIdentityMigrationController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)

	maestroCreateClusterScopedReadonlyBundlesController := controllers.NewCreateClusterScopedMaestroReadonlyBundlesController(
		activeOperationLister, b.options.CosmosDBClient, b.options.ClustersServiceClient,
		backendInformers, b.options.MaestroSourceEnvironmentIdentifier, maestroClientBuilder,
	)
	maestroReadAndPersistClusterScopedReadonlyBundlesContentController := controllers.NewReadAndPersistClusterScopedMaestroReadonlyBundlesContentController(
		activeOperationLister, b.options.CosmosDBClient, b.options.ClustersServiceClient,
		backendInformers, b.options.MaestroSourceEnvironmentIdentifier, maestroClientBuilder,
	)

	maestroCreateNodePoolScopedReadonlyBundlesController := controllers.NewCreateNodePoolScopedMaestroReadonlyBundlesController(
		activeOperationLister, b.options.CosmosDBClient, b.options.ClustersServiceClient,
		backendInformers, b.options.MaestroSourceEnvironmentIdentifier, maestroClientBuilder,
	)
	maestroReadAndPersistNodePoolScopedReadonlyBundlesContentController := controllers.NewReadAndPersistNodePoolScopedMaestroReadonlyBundlesContentController(
		activeOperationLister, b.options.CosmosDBClient, b.options.ClustersServiceClient,
		backendInformers, b.options.MaestroSourceEnvironmentIdentifier, maestroClientBuilder,
	)

	maestroDeleteOrphanedReadonlyBundlesController := controllers.NewDeleteOrphanedMaestroReadonlyBundlesController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		maestroClientBuilder,
		b.options.MaestroSourceEnvironmentIdentifier,
	)

	azureRPRegistrationValidationController := validationcontrollers.NewClusterValidationController(
		validations.NewAzureResourceProvidersRegistrationValidation(b.options.FPAClientBuilder),
		activeOperationLister,
		b.options.CosmosDBClient,
		backendInformers,
	)
	azureClusterResourceGroupExistenceValidationController := validationcontrollers.NewClusterValidationController(
		validations.NewAzureClusterResourceGroupExistenceValidation(b.options.FPAClientBuilder),
		activeOperationLister,
		b.options.CosmosDBClient,
		backendInformers,
	)
	azureClusterManagedIdentitiesExistenceValidationController := validationcontrollers.NewClusterValidationController(
		validations.NewAzureClusterManagedIdentitiesExistenceValidation(b.options.SMIClientBuilder),
		activeOperationLister,
		b.options.CosmosDBClient,
		backendInformers,
	)

	nodePoolVersionController := upgradecontrollers.NewNodePoolVersionController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)

	triggerNodePoolUpgradeController := upgradecontrollers.NewTriggerNodePoolUpgradeController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)

	nodePoolPropertiesSyncController := nodepoolpropertiescontroller.NewNodePoolPropertiesSyncController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)

	nodePoolCustomerPropertiesMigrationController := nodepoolpropertiescontroller.NewNodePoolCustomerPropertiesMigrationController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)

	externalAuthCustomerPropertiesMigrationController := externalauthpropertiescontroller.NewExternalAuthCustomerPropertiesMigrationController(
		b.options.CosmosDBClient,
		b.options.ClustersServiceClient,
		activeOperationLister,
		backendInformers,
	)

	le, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          b.options.LeaderElectionLock,
		LeaseDuration: leaderElectionLeaseDuration,
		RenewDeadline: leaderElectionRenewDeadline,
		RetryPeriod:   leaderElectionRetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// start the SharedInformers
				go backendInformers.RunWithContext(ctx)

				go subscriptionNonClusterDataDumpController.Run(ctx, 20)
				go clusterRecursiveDataDumpController.Run(ctx, 20)
				go csStateDumpController.Run(ctx, 20)
				go billingDumpController.Run(ctx, 20)
				go doNothingController.Run(ctx, 20)
				go operationClusterCreateController.Run(ctx, 20)
				go operationClusterUpdateController.Run(ctx, 20)
				go operationClusterDeleteController.Run(ctx, 20)
				go operationNodePoolCreateController.Run(ctx, 20)
				go operationNodePoolUpdateController.Run(ctx, 20)
				go operationNodePoolDeleteController.Run(ctx, 20)
				go operationExternalAuthCreateController.Run(ctx, 20)
				go operationExternalAuthUpdateController.Run(ctx, 20)
				go operationExternalAuthDeleteController.Run(ctx, 20)
				go operationRequestCredentialController.Run(ctx, 20)
				go operationRevokeCredentialsController.Run(ctx, 20)
				go clusterServiceMatchingClusterController.Run(ctx, 20)
				go cosmosMatchingNodePoolController.Run(ctx, 20)
				go cosmosMatchingExternalAuthController.Run(ctx, 20)
				go cosmosMatchingClusterController.Run(ctx, 20)
				go alwaysSuccessClusterValidationController.Run(ctx, 20)
				go deleteOrphanedCosmosResourcesController.Run(ctx, 20)
				go backfillClusterUIDController.Run(ctx, 20)
				go orphanedBillingCleanupController.Run(ctx, 20)
				go createBillingDocController.Run(ctx, 20)
				go controlPlaneActiveVersionController.Run(ctx, 20)
				go controlPlaneDesiredVersionController.Run(ctx, 20)
				go triggerControlPlaneUpgradeController.Run(ctx, 20)
				go clusterPropertiesSyncController.Run(ctx, 20)
				go clusterServiceMigrationController.Run(ctx, 20)
				go identityMigrationController.Run(ctx, 20)
				go azureRPRegistrationValidationController.Run(ctx, 20)
				go azureClusterResourceGroupExistenceValidationController.Run(ctx, 20)
				go azureClusterManagedIdentitiesExistenceValidationController.Run(ctx, 20)
				go nodePoolVersionController.Run(ctx, 20)
				go maestroCreateClusterScopedReadonlyBundlesController.Run(ctx, 20)
				go maestroReadAndPersistClusterScopedReadonlyBundlesContentController.Run(ctx, 20)
				go maestroCreateNodePoolScopedReadonlyBundlesController.Run(ctx, 20)
				go maestroReadAndPersistNodePoolScopedReadonlyBundlesContentController.Run(ctx, 20)
				go maestroDeleteOrphanedReadonlyBundlesController.Run(ctx, 20)
				go triggerNodePoolUpgradeController.Run(ctx, 20)
				go nodePoolPropertiesSyncController.Run(ctx, 20)
				go nodePoolCustomerPropertiesMigrationController.Run(ctx, 20)
				go externalAuthCustomerPropertiesMigrationController.Run(ctx, 20)
				go operationPhaseMetricsController.Run(ctx, 1)
				go clusterMetricsController.Run(ctx, 1)
				go nodePoolMetricsController.Run(ctx, 1)
				go externalAuthMetricsController.Run(ctx, 1)
			},
			OnStoppedLeading: func() {
				// This needs to be defined even though it does nothing.
			},
		},
		ReleaseOnCancel: true,
		WatchDog:        electionChecker,
		Name:            leaderElectionLockName,
	})
	if err != nil {
		return err
	}

	le.Run(ctx)
	return nil
}
