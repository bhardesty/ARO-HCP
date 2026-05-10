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
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

	"github.com/Azure/ARO-HCP/internal/api"
	"github.com/Azure/ARO-HCP/internal/api/fleet"
	"github.com/Azure/ARO-HCP/internal/utils"
	"github.com/Azure/ARO-HCP/internal/validation"
)

const fleetContainer = "Fleet"

// FleetDBClient is the database surface for the Fleet Cosmos container.
// It is intentionally separate from ResourcesDBClient because the Fleet
// container holds management cluster inventory data with its own access
// patterns and credential scoping.
type FleetDBClient interface {
	Fleet(stampIdentifier string) FleetCRUD
	GlobalListers() FleetGlobalListers
}

// FleetCRUD scopes ResourceCRUD accessors to a single fleet partition (stamp).
// Constructed from FleetDBClient.Fleet(stampIdentifier).
type FleetCRUD interface {
	ManagementClusters() ManagementClustersCRUD
}

// ManagementClustersCRUD provides CRUD operations for management clusters
// and access to their nested controller status documents.
type ManagementClustersCRUD interface {
	ValidatingResourceCRUD[fleet.ManagementCluster]
	Controllers() ResourceCRUD[api.Controller]
}

// FleetGlobalListers provides cross-partition listers for fleet resource types.
type FleetGlobalListers interface {
	ManagementClusters() GlobalLister[fleet.ManagementCluster]
}

type cosmosFleetDBClient struct {
	container *azcosmos.ContainerClient
}

var _ FleetDBClient = &cosmosFleetDBClient{}

// NewFleetDBClient instantiates a FleetDBClient from a Cosmos DatabaseClient.
func NewFleetDBClient(database *azcosmos.DatabaseClient) (FleetDBClient, error) {
	container, err := database.NewContainer(fleetContainer)
	if err != nil {
		return nil, utils.TrackError(err)
	}
	return &cosmosFleetDBClient{container: container}, nil
}

// NewFleetDBClientFromContainer wraps an already-opened container client.
func NewFleetDBClientFromContainer(container *azcosmos.ContainerClient) FleetDBClient {
	return &cosmosFleetDBClient{container: container}
}

func (c *cosmosFleetDBClient) Fleet(stampIdentifier string) FleetCRUD {
	return &cosmosFleetCRUD{
		containerClient: c.container,
		stampIdentifier: stampIdentifier,
	}
}

func (c *cosmosFleetDBClient) GlobalListers() FleetGlobalListers {
	return &cosmosFleetGlobalListers{container: c.container}
}

// cosmosFleetCRUD implements FleetCRUD against a Cosmos container.
type cosmosFleetCRUD struct {
	containerClient *azcosmos.ContainerClient
	stampIdentifier string
}

var _ FleetCRUD = &cosmosFleetCRUD{}

func (k *cosmosFleetCRUD) ManagementClusters() ManagementClustersCRUD {
	inner := newFleetResourceCRUD[fleet.ManagementCluster, GenericDocument[fleet.ManagementCluster]](
		k.containerClient, k.stampIdentifier, fleet.ManagementClusterResourceType,
	)
	return &cosmosManagementClustersCRUD{
		ValidatingResourceCRUD: NewValidatingCRUD(inner,
			validation.ValidateManagementClusterCreate,
			validation.ValidateManagementClusterUpdate,
		),
		containerClient: k.containerClient,
		stampIdentifier: k.stampIdentifier,
	}
}

type cosmosManagementClustersCRUD struct {
	ValidatingResourceCRUD[fleet.ManagementCluster]
	containerClient *azcosmos.ContainerClient
	stampIdentifier string
}

func (m *cosmosManagementClustersCRUD) Controllers() ResourceCRUD[api.Controller] {
	mcResourceID, err := fleet.ToManagementClusterResourceID(m.stampIdentifier)
	if err != nil {
		panic(fmt.Sprintf("invalid stamp identifier %q: %v", m.stampIdentifier, err))
	}
	return &fleetResourceCRUD[api.Controller, GenericDocument[api.Controller]]{
		containerClient:  m.containerClient,
		parentResourceID: mcResourceID,
		resourceType:     fleet.ManagementClusterControllerResourceType,
		partitionKey:     strings.ToLower(m.stampIdentifier),
	}
}

type cosmosFleetGlobalListers struct {
	container *azcosmos.ContainerClient
}

var _ FleetGlobalListers = &cosmosFleetGlobalListers{}

func (g *cosmosFleetGlobalListers) ManagementClusters() GlobalLister[fleet.ManagementCluster] {
	return &cosmosGlobalLister[fleet.ManagementCluster, GenericDocument[fleet.ManagementCluster]]{
		containerClient: g.container,
		resourceType:    fleet.ManagementClusterResourceType,
	}
}
