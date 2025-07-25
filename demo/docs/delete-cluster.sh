#!/bin/bash
# This script deletes an ARO HCP cluster and waits for its complete deletion.

source .env

echo "Initiating deletion of HCP cluster: $CLUSTER_NAME in resource group: $CUSTOMER_RG_NAME..."
az rest \
  --method DELETE \
  --uri "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${CUSTOMER_RG_NAME}/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/${CLUSTER_NAME}?api-version=2024-06-10-preview" \
  --output none

echo "Delete request sent. Starting to poll for deletion status..."

# --- Poll for deletion status ---
STATUS=""
POLLING_INTERVAL=30 # seconds to wait between checks

while true; do
  echo "Checking status of cluster $CLUSTER_NAME..."
  
  # Attempt to GET the cluster status. Redirect stderr to stdout so we can grep for errors.
  CLUSTER_STATUS_JSON=$(az rest \
    --method GET \
    --uri "/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${CUSTOMER_RG_NAME}/providers/Microsoft.RedHatOpenShift/hcpOpenShiftClusters/${CLUSTER_NAME}?api-version=2024-06-10-preview" \
    --output json 2>&1) # Capture stdout AND stderr

  # Check the exit code of the previous az rest command
  if [ $? -ne 0 ]; then
    # If az rest failed, check if it's because the resource was not found
    if echo "$CLUSTER_STATUS_JSON" | grep -q "ResourceNotFound"; then
      echo "✅ Cluster '$CLUSTER_NAME' successfully deleted (resource not found)."
      break # Exit the loop, deletion is complete
    else
      echo "❌ An unexpected error occurred while checking cluster status:"
      echo "$CLUSTER_STATUS_JSON"
      exit 1 # Exit script on unexpected error
    fi
  fi

  # If the GET call was successful (resource still exists), parse the provisioningState
  STATUS=$(echo "$CLUSTER_STATUS_JSON" | jq -r '.properties.provisioningState')

  if [ "$STATUS" == "Deleting" ]; then
    echo "Status: $STATUS. Waiting for completion..."
    sleep $POLLING_INTERVAL
  elif [ "$STATUS" == "Succeeded" ] || [ "$STATUS" == "Failed" ]; then
    # This case is less common for deletions (usually goes to Not Found)
    # but good for robustness if the API returns other states.
    echo "Status: $STATUS. Deletion process completed (or failed)."
    break
  else
    echo "Unknown status: '$STATUS'. Waiting for completion or 'Not Found'..."
    sleep $POLLING_INTERVAL
  fi

done
