#!/bin/bash
set -e

# Configuration
GATEWAY_URL="http://localhost:8080"
ADMIN_API_URL="http://localhost:8081"
ADMIN_USER="sushi-admin"
ADMIN_PASSWORD="sushi-password"

echo "=== Dynamic Configuration Verification ==="

# 1. Login to get JWT Token
echo "Logging in..."
# Login using Basic Auth and capture cookies
COOKIE_FILE=$(mktemp)
curl -s -X POST "$ADMIN_API_URL/login" \
  -u "$ADMIN_USER:$ADMIN_PASSWORD" \
  -c "$COOKIE_FILE" \
  -o /dev/null

# Extract token from cookie file
JWT_TOKEN=$(grep "token" "$COOKIE_FILE" | awk '{print $7}')

if [ -z "$JWT_TOKEN" ]; then
  echo "Failed to get JWT token from cookie"
  cat "$COOKIE_FILE"
  rm "$COOKIE_FILE"
  exit 1
fi

rm "$COOKIE_FILE"
echo "JWT Token obtained."

# 2. List current services
echo "Listing services..."
curl -s -X GET "$ADMIN_API_URL/services" -H "Authorization: Bearer $JWT_TOKEN" | jq .

# 3. Add a new service
echo "Adding test service 'dynamic-echo'..."
curl -v -s -X POST "$ADMIN_API_URL/services" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dynamic-echo",
    "base_path": "/echo",
    "protocol": "http",
    "host": "httpbin.org",
    "upstreams": [{"target": "httpbin.org:80"}]
  }'

# 4. Add a route for the service
echo "Adding route for 'dynamic-echo'..."
curl -v -s -X POST "$ADMIN_API_URL/services/dynamic-echo/routes" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "echo-route",
    "paths": ["/test"],
    "strip_path": false
  }'

# 5. Verify service is active (proxying)
echo "Verifying proxying to /echo/test..."
PROXY_RESPONSE=$(curl -s -v -o /dev/null -w "%{http_code}" "$GATEWAY_URL/echo/test")

if [ "$PROXY_RESPONSE" != "200" ]; then
  echo "FAILURE: Proxy returned $PROXY_RESPONSE"
  # Let's see why it failed
  curl -v "$GATEWAY_URL/echo/test"
else
  echo "SUCCESS: Proxying works!"
fi

# 6. Delete the test service
echo "Deleting test service 'dynamic-echo'..."
# Deleting service should cascading delete routes in real Kong, 
# for now we'll just delete the service.
curl -s -X DELETE "$ADMIN_API_URL/services/dynamic-echo" \
  -H "Authorization: Bearer $JWT_TOKEN"

echo "Verification complete."
