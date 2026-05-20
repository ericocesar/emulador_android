#!/bin/bash

set -e

# Required environment variables:
# PORTAINER_URL
# PORTAINER_API_KEY
# PORTAINER_ENDPOINT_ID
# STACK_NAME
# STACK_FILE
# IMAGE_TAG

if [ -z "$PORTAINER_URL" ] || [ -z "$PORTAINER_API_KEY" ] || [ -z "$PORTAINER_ENDPOINT_ID" ] || [ -z "$STACK_NAME" ] || [ -z "$STACK_FILE" ] || [ -z "$IMAGE_TAG" ]; then
  echo "Error: One or more required environment variables are missing."
  echo "Required: PORTAINER_URL, PORTAINER_API_KEY, PORTAINER_ENDPOINT_ID, STACK_NAME, STACK_FILE, IMAGE_TAG"
  exit 1
fi

echo "Deploying stack $STACK_NAME to Portainer at $PORTAINER_URL..."

# Read stack file content
STACK_CONTENT=$(cat "$STACK_FILE")

# Get Stack ID
# We fetch stacks and filter client-side with jq to ensure exact match
# The API filter might be fuzzy or ignored in some contexts
RESPONSE=$(curl -s -G -H "X-API-Key: $PORTAINER_API_KEY" \
  --data-urlencode "filters={\"name\":\"$STACK_NAME\"}" \
  "$PORTAINER_URL/api/stacks")

# Check if response is an array
if ! echo "$RESPONSE" | jq -e 'if type=="array" then true else false end' > /dev/null; then
  echo "Error: Failed to list stacks. Portainer API response:"
  echo "$RESPONSE"
  exit 1
fi

# Find the stack with the exact name
STACK_ID=$(echo "$RESPONSE" | jq -r --arg name "$STACK_NAME" '.[] | select(.Name == $name) | .Id')

if [ -z "$STACK_ID" ]; then
  echo "Stack $STACK_NAME not found. Creating new stack..."
  
  # Get Swarm ID from endpoint
  echo "Fetching Swarm ID from endpoint..."
  ENDPOINT_INFO=$(curl -s -H "X-API-Key: $PORTAINER_API_KEY" \
    "$PORTAINER_URL/api/endpoints/$PORTAINER_ENDPOINT_ID/docker/swarm")
  
  SWARM_ID=$(echo "$ENDPOINT_INFO" | jq -r '.ID // empty')
  
  if [ -z "$SWARM_ID" ]; then
    echo "Error: Unable to get Swarm ID from endpoint"
    echo "Endpoint info:"
    echo "$ENDPOINT_INFO"
    exit 1
  fi
  
  echo "Swarm ID: $SWARM_ID"
  
  # Create Stack
  # Endpoint: POST /api/stacks/create/swarm/string?endpointId={endpointId}
  
  PAYLOAD=$(jq -n \
    --arg name "$STACK_NAME" \
    --arg content "$STACK_CONTENT" \
    --arg image_tag "$IMAGE_TAG" \
    --arg swarm_id "$SWARM_ID" \
    '{
      name: $name,
      stackFileContent: $content,
      swarmID: $swarm_id,
      env: [
        {name: "IMAGE_TAG", value: $image_tag}
      ]
    }')

  echo "Creating stack with payload..."
  echo "Endpoint: $PORTAINER_URL/api/stacks/create/swarm/string?endpointId=$PORTAINER_ENDPOINT_ID"
  
  CREATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST -H "X-API-Key: $PORTAINER_API_KEY" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$PORTAINER_URL/api/stacks/create/swarm/string?endpointId=$PORTAINER_ENDPOINT_ID")
  
  HTTP_CODE=$(echo "$CREATE_RESPONSE" | tail -n1)
  RESPONSE_BODY=$(echo "$CREATE_RESPONSE" | sed '$d')
  
  if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
    echo "Stack created successfully."
    echo "$RESPONSE_BODY"
  else
    echo "Error creating stack. HTTP Code: $HTTP_CODE"
    echo "Response:"
    echo "$RESPONSE_BODY"
    exit 1
  fi

else
  echo "Stack $STACK_NAME found (ID: $STACK_ID). Updating..."
  
  # Update Stack
  # Endpoint: PUT /api/stacks/{id}?endpointId={endpointId}
  
  PAYLOAD=$(jq -n \
    --arg content "$STACK_CONTENT" \
    --arg image_tag "$IMAGE_TAG" \
    '{
      stackFileContent: $content,
      env: [
        {name: "IMAGE_TAG", value: $image_tag}
      ],
      prune: true,
      pullImage: true
    }')

  echo "Updating stack with ID $STACK_ID..."
  
  UPDATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X PUT -H "X-API-Key: $PORTAINER_API_KEY" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$PORTAINER_URL/api/stacks/$STACK_ID?endpointId=$PORTAINER_ENDPOINT_ID")
  
  HTTP_CODE=$(echo "$UPDATE_RESPONSE" | tail -n1)
  RESPONSE_BODY=$(echo "$UPDATE_RESPONSE" | sed '$d')
  
  if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
    echo "Stack updated successfully."
    echo "$RESPONSE_BODY"
  else
    echo "Error updating stack. HTTP Code: $HTTP_CODE"
    echo "Response:"
    echo "$RESPONSE_BODY"
    exit 1
  fi
fi
