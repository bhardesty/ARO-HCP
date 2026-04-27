// Copyright 2025 Microsoft Corporation
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

package operationcontrollers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/blang/semver/v4"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/Azure/ARO-HCP/backend/pkg/controllers/controllerutils"
	"github.com/Azure/ARO-HCP/internal/api"
	"github.com/Azure/ARO-HCP/internal/api/arm"
	"github.com/Azure/ARO-HCP/internal/database"
	"github.com/Azure/ARO-HCP/internal/ocm"
	"github.com/Azure/ARO-HCP/internal/utils"
)

type operationClusterUpdate struct {
	cosmosClient         database.DBClient
	clusterServiceClient ocm.ClusterServiceClientSpec
	notificationClient   *http.Client
}

// NewOperationClusterUpdateController periodically lists all clusters and for each out when the cluster was created and its state.
func NewOperationClusterUpdateController(
	cosmosClient database.DBClient,
	clusterServiceClient ocm.ClusterServiceClientSpec,
	notificationClient *http.Client,
	activeOperationInformer cache.SharedIndexInformer,
) controllerutils.Controller {
	syncer := &operationClusterUpdate{
		cosmosClient:         cosmosClient,
		clusterServiceClient: clusterServiceClient,
		notificationClient:   notificationClient,
	}

	controller := NewGenericOperationController(
		"OperationClusterUpdate",
		syncer,
		10*time.Second,
		activeOperationInformer,
		cosmosClient,
	)

	return controller
}

func (c *operationClusterUpdate) ShouldProcess(ctx context.Context, operation *api.Operation) bool {
	if operation.Status.IsTerminal() {
		return false
	}
	if operation.Request != database.OperationRequestUpdate {
		return false
	}
	if operation.ExternalID == nil || !strings.EqualFold(operation.ExternalID.ResourceType.String(), api.ClusterResourceType.String()) {
		return false
	}
	return true
}

func (c *operationClusterUpdate) SynchronizeOperation(ctx context.Context, key controllerutils.OperationKey) error {
	logger := utils.LoggerFromContext(ctx)
	logger.Info("checking operation")

	operation, err := c.cosmosClient.Operations(key.SubscriptionID).Get(ctx, key.OperationName)
	if database.IsNotFoundError(err) {
		return nil // no work to do
	}
	if err != nil {
		return fmt.Errorf("failed to get active operation: %w", err)
	}
	if !c.ShouldProcess(ctx, operation) {
		return nil // no work to do
	}
	if len(operation.InternalID.String()) == 0 {
		// we cannot proceed: yet.
		// TODO when we update to make clusterserice creation async, we need to handle this correctly.
		return nil
	}

	operationalState, err := c.determineOperationState(ctx, operation)
	if err != nil {
		return utils.TrackError(err)
	}

	var persistErr *arm.CloudErrorBody
	if operationalState.provisioningState == arm.ProvisioningStateFailed {
		persistErr = &arm.CloudErrorBody{
			Code:    arm.CloudErrorCodeInvalidRequestContent,
			Message: operationalState.message,
		}
	}

	logger.Info("updating status")
	if err := UpdateOperationStatus(ctx, c.cosmosClient, operation, operationalState.provisioningState, persistErr, postAsyncNotificationFn(c.notificationClient)); err != nil {
		return utils.TrackError(err)
	}
	return nil
}

func (c *operationClusterUpdate) determineOperationState(ctx context.Context, operation *api.Operation) (*operationState, error) {
	logger := utils.LoggerFromContext(ctx)
	errs := []error{}
	operationStates := []*operationState{}

	if operationState, err := c.desiredVersionResolutionOperationState(ctx, operation); err != nil {
		errs = append(errs, utils.TrackError(err))
	} else {
		operationStates = append(operationStates, operationState)
	}
	if operationState, csErr := c.clusterServiceUpdateOperationState(ctx, operation); csErr != nil {
		errs = append(errs, utils.TrackError(csErr))
	} else {
		operationStates = append(operationStates, operationState)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if len(operationStates) == 0 {
		return nil, utils.TrackError(fmt.Errorf("no operation states"))
	}
	slices.SortStableFunc(operationStates, compareOperationState)
	if operationStates[0] == nil {
		return nil, errors.New("nil operation state")
	}
	logger.Info("determined cluster update operation status", "operationStates", operationStates)
	picked, err := pickWorstOperationState(operationStates)
	if err != nil {
		return nil, utils.TrackError(err)
	}
	logger.Info("picked cluster update operation status", "provisioningState", picked.provisioningState, "message", picked.message)
	return picked, nil
}

func (c *operationClusterUpdate) desiredVersionResolutionOperationState(ctx context.Context, operation *api.Operation) (*operationState, error) {
	existingCluster, err := c.cosmosClient.HCPClusters(operation.ExternalID.SubscriptionID, operation.ExternalID.ResourceGroupName).Get(ctx, operation.ExternalID.Name)
	if err != nil {
		return nil, utils.TrackError(err)
	}
	existingServiceProviderCluster, err := c.cosmosClient.ServiceProviderClusters(operation.ExternalID.SubscriptionID, operation.ExternalID.ResourceGroupName, operation.ExternalID.Name).Get(ctx, api.ServiceProviderClusterResourceName)
	if err != nil {
		return nil, utils.TrackError(err)
	}
	resultingDesiredVersion := existingServiceProviderCluster.Spec.ControlPlaneVersion.DesiredVersion
	if resultingDesiredVersion == nil {
		return nil, utils.TrackError(fmt.Errorf("service provider cluster has no desired version"))
	}

	customerDesiredVersion, err := semver.ParseTolerant(existingCluster.CustomerProperties.Version.ID)
	if err != nil {
		return nil, utils.TrackError(err)
	}

	if customerDesiredVersion.Major == resultingDesiredVersion.Major &&
		customerDesiredVersion.Minor == resultingDesiredVersion.Minor {
		return newOperationState(arm.ProvisioningStateSucceeded, ""), nil
	}
	existingDegraded := apimeta.FindStatusCondition(existingServiceProviderCluster.Status.Conditions,
		api.DegradedCondition)
	if existingDegraded != nil && existingDegraded.Status == metav1.ConditionTrue &&
		existingDegraded.Reason == api.ServiceProviderClusterConditionReasonVersionUpgradeNotAccepted {
		return newOperationState(arm.ProvisioningStateFailed, existingDegraded.Message), nil
	}
	return nil, utils.TrackError(fmt.Errorf("customer desired version does not match resolved desired version"))
}

func (c *operationClusterUpdate) clusterServiceUpdateOperationState(ctx context.Context, operation *api.Operation) (*operationState, error) {
	logger := utils.LoggerFromContext(ctx)
	clusterStatus, err := c.clusterServiceClient.GetClusterStatus(ctx, operation.InternalID)
	if err != nil {
		return nil, utils.TrackError(err)
	}
	newOperationStatus, opError, err := convertClusterStatus(ctx, c.clusterServiceClient, operation, clusterStatus)
	if err != nil {
		return nil, utils.TrackError(err)
	}
	logger.Info("new status via cluster-service", "newStatus", newOperationStatus, "newOperationError", opError)
	msg := ""
	if opError != nil {
		msg = opError.Message
	}
	return newOperationState(newOperationStatus, msg), nil
}
