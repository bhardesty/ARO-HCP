#!/bin/bash
# This script requests a temporary admin credential for an ARO HCP cluster.

source .env

az rest \
  --method POST \
  --uri /subscriptions/{subscriptionId}/resourceGroups/$CUSTOMER_RG_NAME/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/$CLUSTER_NAME/requestAdminCredential?api-version=2024-06-10-preview \
  --verbose
  