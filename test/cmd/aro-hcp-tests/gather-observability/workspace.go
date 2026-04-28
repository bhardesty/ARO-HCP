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

package gatherobservability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azcorearm "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
)

const (
	workspaceSvc = "svc"
	workspaceHcp = "hcp"
)

type workspaceData struct {
	Type         string
	PromEndpoint string
	AlertRules   []string
	FiredAlerts  []alert
}

func fetchWorkspaceData(ctx context.Context, cred azcore.TokenCredential, wsType string, workspaceResourceID azcorearm.ResourceID, start, end time.Time, severityThreshold int, knownIssues []knownIssue) (*workspaceData, error) {
	logger, err := logr.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("logger not found in context: %w", err)
	}

	promEndpoint, err := lookupPrometheusEndpoint(ctx, cred, workspaceResourceID.SubscriptionID, workspaceResourceID.ResourceGroupName, workspaceResourceID.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to look up Prometheus endpoint: %w", err)
	}
	logger.Info("resolved Prometheus endpoint", "workspace", wsType, "endpoint", promEndpoint)

	rules, err := fetchAlertRules(ctx, cred, workspaceResourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch alert rules: %w", err)
	}
	logger.Info("fetched alert rules", "workspace", wsType, "count", len(rules))

	scope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", workspaceResourceID.SubscriptionID, workspaceResourceID.ResourceGroupName)
	allAlerts, err := fetchAlerts(ctx, cred, scope, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch alerts: %w", err)
	}

	var alerts []alert
	for _, a := range allAlerts {
		if alertBelongsToWorkspace(a, workspaceResourceID) {
			a.Metadata.MonitoringWorkspaceType = wsType
			alerts = append(alerts, a)
		}
	}

	alerts = filterAlertsBySeverity(alerts, severityThreshold)
	alerts = classifyAlerts(alerts, knownIssues)
	logger.Info("fetched fired alerts", "workspace", wsType, "count", len(alerts))

	return &workspaceData{
		Type:         wsType,
		PromEndpoint: promEndpoint,
		AlertRules:   rules,
		FiredAlerts:  alerts,
	}, nil
}

func alertBelongsToWorkspace(a alert, ws azcorearm.ResourceID) bool {
	return strings.EqualFold(a.Metadata.MonitoringWorkspace, ws.String())
}
