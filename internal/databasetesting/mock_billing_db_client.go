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

package databasetesting

import (
	"github.com/Azure/ARO-HCP/internal/database"
)

// MockBillingDBClient implements database.BillingDBClient using the same in-memory
// billing partition as mockResourcesDBClient.
type MockBillingDBClient struct {
	arm *MockResourcesDBClient
}

var _ database.BillingDBClient = (*MockBillingDBClient)(nil)

// NewMockBillingDBClient returns a BillingDBClient backed by the given mock ARM client’s billing store.
func NewMockBillingDBClient(arm *MockResourcesDBClient) *MockBillingDBClient {
	return &MockBillingDBClient{arm: arm}
}

func (m *MockBillingDBClient) BillingDocs(subscriptionID string) database.BillingDocCRUD {
	return newMockBillingDocCRUD(m.arm, subscriptionID)
}

func (m *MockBillingDBClient) BillingGlobalListers() database.BillingGlobalListers {
	return &mockBillingDBGlobalListers{arm: m.arm}
}

type mockBillingDBGlobalListers struct {
	arm *MockResourcesDBClient
}

var _ database.BillingGlobalListers = (*mockBillingDBGlobalListers)(nil)

func (g *mockBillingDBGlobalListers) BillingDocs() database.GlobalLister[database.BillingDocument] {
	return &mockBillingGlobalLister{client: g.arm}
}
