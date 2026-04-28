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
	"fmt"
	"testing"

	azcorearm "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
)

func mustParseResourceID(sub, rg, name string) *azcorearm.ResourceID {
	id, err := azcorearm.ParseResourceID(fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Monitor/accounts/%s", sub, rg, name))
	if err != nil {
		panic(err)
	}
	return id
}

func TestAlertBelongsToWorkspace(t *testing.T) {
	t.Parallel()
	ws := mustParseResourceID("sub-123", "my-rg", "services-westus3")

	tests := []struct {
		name                string
		monitoringWorkspace string
		want                bool
	}{
		{
			name:                "matching_workspace",
			monitoringWorkspace: ws.String(),

			want: true,
		},
		{
			name:                "different_workspace",
			monitoringWorkspace: mustParseResourceID("sub-123", "my-rg", "hcps-westus3").String(),
			want:                false,
		},
		{
			name:                "empty_no_match",
			monitoringWorkspace: "",
			want:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := alert{Metadata: alertMetadata{MonitoringWorkspace: tt.monitoringWorkspace}}
			got := alertBelongsToWorkspace(a, *ws)
			if got != tt.want {
				t.Errorf("alertBelongsToWorkspace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScopeContainsWorkspace(t *testing.T) {
	t.Parallel()
	wsPtr := mustParseResourceID("sub-123", "my-rg", "hcps-westus3")
	wsStr := wsPtr.String()

	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name   string
		scopes []*string
		want   bool
	}{
		{
			name:   "matching_scope",
			scopes: []*string{&wsStr},
			want:   true,
		},
		{
			name:   "no_match",
			scopes: []*string{strPtr(mustParseResourceID("sub-123", "my-rg", "services-westus3").String())},
			want:   false,
		},
		{
			name:   "nil_scope_skipped",
			scopes: []*string{nil, &wsStr},
			want:   true,
		},
		{
			name:   "empty_scopes",
			scopes: []*string{},
			want:   false,
		},
		{
			name:   "nil_scopes",
			scopes: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := scopeContainsWorkspace(tt.scopes, *wsPtr)
			if got != tt.want {
				t.Errorf("scopeContainsWorkspace() = %v, want %v", got, tt.want)
			}
		})
	}
}
