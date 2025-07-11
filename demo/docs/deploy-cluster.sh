#!/bin/bash
# This script creates an ARO HCP cluster and a single node pool.

source .env

az group create \
  --name "${CUSTOMER_RG_NAME}" \
  --subscription "${SUBSCRIPTION}" \
  --location "${LOCATION}"

az policy definition create -n $POLICY_DEFINITION \
  --mode All \
  --rules rules.json \
  --params param-defs.json

az policy assignment create -n $POLICY_ASSIGNMENT \
  --policy $POLICY_DEFINITION \
  --scope "/subscriptions/${SUBSCRIPTION_ID}" \
  --location $LOCATION \
  --mi-system-assigned \
  --role "Tag Contributor" \
  --identity-scope "/subscriptions/${SUBSCRIPTION_ID}" \
  --params param-values.json

az deployment group create \
  --name 'aro-hcp' \
  --subscription "${SUBSCRIPTION}" \
  --resource-group "${CUSTOMER_RG_NAME}" \
  --template-file azuredeploy.bicep \
  --parameters \
    customerNsgName="${CUSTOMER_NSG}" \
    customerVnetName="${CUSTOMER_VNET_NAME}" \
    customerVnetSubnetName="${CUSTOMER_VNET_SUBNET1}" \
    clusterName="${CLUSTER_NAME}" \
    managedResourceGroupName="${MANAGED_RESOURCE_GROUP}" \
    nodePoolName="${NP_NAME}"
