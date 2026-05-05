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

package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestARMResourcesGlobalListers_Billing verifies that the ARMResourcesGlobalListers interface
// provides access to a billing document global lister.
func TestARMResourcesGlobalListers_Billing(t *testing.T) {
	// Create a cosmosARMResourcesGlobalListers instance with nil containers
	// (we're only testing the interface, not actual Cosmos DB interaction)
	gl := &cosmosARMResourcesGlobalListers{
		resources: nil,
		billing:   nil,
	}

	// Verify the BillingDocs method exists and returns a GlobalLister
	lister := gl.BillingDocs()
	require.NotNil(t, lister, "BillingDocs() should return a non-nil GlobalLister")
}
