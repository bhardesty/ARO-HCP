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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

	"github.com/Azure/ARO-HCP/internal/api/arm"
)

type partitionKeySetter interface {
	SetPartitionKey(pk string)
}

func (td *TypedDocument) SetPartitionKey(pk string) {
	td.PartitionKey = pk
}

// serializeFleetItem serializes an object for the Fleet Cosmos container.
// The partition key is provided by the CRUD layer rather than extracted from
// the object, so any type that implements CosmosPersistable can be stored in
// the Fleet container regardless of whether it carries fleet-specific accessors.
func serializeFleetItem[InternalAPIType, CosmosAPIType any](
	partitionKeyString string,
	newObj *InternalAPIType,
) (*arm.CosmosMetadata, []byte, error) {
	cosmosPersistable, ok := any(newObj).(arm.CosmosPersistable)
	if !ok {
		return nil, nil, fmt.Errorf("type %T does not implement CosmosPersistable interface", newObj)
	}
	cosmosData := cosmosPersistable.GetCosmosData()
	cosmosUID := cosmosData.GetCosmosUID()
	if len(cosmosUID) == 0 {
		return nil, nil, fmt.Errorf("no cosmos id found in object")
	}
	if !strings.EqualFold(cosmosUID, strings.ToLower(cosmosUID)) {
		return nil, nil, fmt.Errorf("invalid cosmos id found in object")
	}

	cosmosObj, err := InternalToCosmos[InternalAPIType, CosmosAPIType](newObj)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert internal object to Cosmos object: %w", err)
	}

	// The conversion layer may have set the wrong partition key (e.g.
	// subscription ID for generic types). Override with the CRUD's
	// partition key which is always the stamp identifier for Fleet.
	// replace this with the functionality that will be introduced
	// by https://github.com/Azure/ARO-HCP/pull/5094
	if doc, ok := any(cosmosObj).(partitionKeySetter); ok {
		doc.SetPartitionKey(partitionKeyString)
	}

	data, err := json.Marshal(cosmosObj)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal Cosmos DB item for '%s': %w", cosmosData.ResourceID, err)
	}

	return cosmosData, data, nil
}

func createFleetItem[InternalAPIType, CosmosAPIType any](
	ctx context.Context,
	containerClient *azcosmos.ContainerClient,
	partitionKeyString string,
	newObj *InternalAPIType,
	opts *azcosmos.ItemOptions,
) (*InternalAPIType, error) {
	if strings.ToLower(partitionKeyString) != partitionKeyString {
		return nil, fmt.Errorf("partitionKeyString must be lowercase, not: %q", partitionKeyString)
	}
	cosmosMetadata, data, err := serializeFleetItem[InternalAPIType, CosmosAPIType](partitionKeyString, newObj)
	if err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &azcosmos.ItemOptions{}
	}
	opts.EnableContentResponseOnWrite = true

	responseItem, err := containerClient.CreateItem(ctx, azcosmos.NewPartitionKeyString(partitionKeyString), data, opts)
	if err != nil {
		return nil, err
	}

	return responseItemToInternalObj[InternalAPIType, CosmosAPIType](ctx, cosmosMetadata.GetCosmosUID(), responseItem)
}

func replaceFleetItem[InternalAPIType, CosmosAPIType any](
	ctx context.Context,
	containerClient *azcosmos.ContainerClient,
	partitionKeyString string,
	newObj *InternalAPIType,
	opts *azcosmos.ItemOptions,
) (*InternalAPIType, error) {
	if strings.ToLower(partitionKeyString) != partitionKeyString {
		return nil, fmt.Errorf("partitionKeyString must be lowercase, not: %q", partitionKeyString)
	}
	cosmosMetadata, data, err := serializeFleetItem[InternalAPIType, CosmosAPIType](partitionKeyString, newObj)
	if err != nil {
		return nil, err
	}

	if opts == nil {
		opts = &azcosmos.ItemOptions{}
	}
	if len(cosmosMetadata.CosmosETag) > 0 {
		opts.IfMatchEtag = &cosmosMetadata.CosmosETag
	}
	opts.EnableContentResponseOnWrite = true

	responseItem, err := containerClient.ReplaceItem(
		ctx, azcosmos.NewPartitionKeyString(partitionKeyString), cosmosMetadata.GetCosmosUID(), data, opts,
	)
	if err != nil {
		return nil, err
	}

	return responseItemToInternalObj[InternalAPIType, CosmosAPIType](ctx, cosmosMetadata.GetCosmosUID(), responseItem)
}

func addFleetCreateToTransaction[InternalAPIType, CosmosAPIType any](
	ctx context.Context,
	transaction DBTransaction,
	newObj *InternalAPIType,
	opts *azcosmos.TransactionalBatchItemOptions,
) (string, error) {
	partitionKeyString := transaction.GetPartitionKey()
	if strings.ToLower(partitionKeyString) != partitionKeyString {
		return "", fmt.Errorf("partitionKeyString must be lowercase, not: %q", partitionKeyString)
	}
	cosmosMetadata, data, err := serializeFleetItem[InternalAPIType, CosmosAPIType](partitionKeyString, newObj)
	if err != nil {
		return "", err
	}
	transactionDetails := CosmosDBTransactionStepDetails{
		ActionType: "Create",
		GoType:     fmt.Sprintf("%T", newObj),
		CosmosID:   cosmosMetadata.GetCosmosUID(),
		ResourceID: cosmosMetadata.ResourceID.String(),
	}

	transaction.AddStep(
		transactionDetails,
		func(b *azcosmos.TransactionalBatch) (string, error) {
			b.CreateItem(data, opts)
			return cosmosMetadata.GetCosmosUID(), nil
		},
	)

	return cosmosMetadata.GetCosmosUID(), nil
}

func addFleetReplaceToTransaction[InternalAPIType, CosmosAPIType any](
	ctx context.Context,
	transaction DBTransaction,
	newObj *InternalAPIType,
	opts *azcosmos.TransactionalBatchItemOptions,
) (string, error) {
	partitionKeyString := transaction.GetPartitionKey()
	if strings.ToLower(partitionKeyString) != partitionKeyString {
		return "", fmt.Errorf("partitionKeyString must be lowercase, not: %q", partitionKeyString)
	}
	cosmosMetadata, data, err := serializeFleetItem[InternalAPIType, CosmosAPIType](partitionKeyString, newObj)
	if err != nil {
		return "", err
	}
	transactionDetails := CosmosDBTransactionStepDetails{
		ActionType: "Replace",
		GoType:     fmt.Sprintf("%T", newObj),
		CosmosID:   cosmosMetadata.GetCosmosUID(),
		ResourceID: cosmosMetadata.ResourceID.String(),
		Etag:       cosmosMetadata.CosmosETag,
	}

	if opts == nil {
		opts = &azcosmos.TransactionalBatchItemOptions{}
	}
	if len(cosmosMetadata.CosmosETag) > 0 {
		opts.IfMatchETag = &cosmosMetadata.CosmosETag
	}

	transaction.AddStep(
		transactionDetails,
		func(b *azcosmos.TransactionalBatch) (string, error) {
			b.ReplaceItem(cosmosMetadata.GetCosmosUID(), data, opts)
			return cosmosMetadata.GetCosmosUID(), nil
		},
	)

	return cosmosMetadata.GetCosmosUID(), nil
}
