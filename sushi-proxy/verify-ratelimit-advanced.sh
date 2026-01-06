#!/bin/bash
set -e

PROXY_PID=""

# Function to start proxy
function start_proxy {
  echo ">>> Starting Proxy with REDIS_ADDR=localhost:6379..."
  rm -f sushi-gateway.db
  # Use INFO log level to reduce noise
  LOG_LEVEL=INFO CONFIG_FILE_PATH=config/config.ratelimit.yaml REDIS_ADDR=localhost:6379 ADMIN_USER=sushi-admin ADMIN_PASSWORD=sushi-password JWT_SECRET=supersecret ./sushi-proxy > proxy_test.log 2>&1 &
  PROXY_PID=$!
  sleep 5 # Wait for startup (fast now)
}

function stop_proxy {
  echo ">>> Stopping Proxy..."
  if [ -n "$PROXY_PID" ]; then kill $PROXY_PID || true; fi
  pkill sushi-proxy || true
  wait $PROXY_PID 2>/dev/null || true
  rm -f sushi-gateway.db
  
  # Ensure Redis is running
  echo ">>> Ensuring Redis is running..."
  docker start sushi-redis >/dev/null 2>&1 || true
}

trap stop_proxy EXIT

# Make sure Redis is running initially
docker start sushi-redis >/dev/null 2>&1 || true
# Wait for Redis to accept connections
sleep 2

start_proxy

# Login
echo ">>> Logging in..."
TOKEN=$(curl -s -X POST http://localhost:8081/login -u sushi-admin:sushi-password | jq -r .token)

# Add Service
echo ">>> Adding Service 'rl-test'..."
curl -s -X POST http://localhost:8081/services -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{
  "name": "rl-test", "url": "https://httpbin.org/get", "protocol": "http", "base_path": "/"
}' > /dev/null

# ==============================================================================
# TEST 1: FAULT TOLERANCE
# ==============================================================================

# Add Route with fault_tolerant=true (Default)
echo ">>> Adding Route 'rl-route' (fault_tolerant=true)..."
curl -s -X POST http://localhost:8081/services/rl-test/routes -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{
  "name": "rl-route", "paths": ["/rl-test"],
  "plugins": [{
    "name": "rate-limiting",
    "config": { "second": 5, "fault_tolerant": true }
  }]
}' > /dev/null

sleep 1

# Verify Normal Operation (200 OK)
echo ">>> Verifying Normal Operation..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/rl-test)
if [ "$STATUS" != "200" ]; then echo "FAIL: Expected 200, got $STATUS"; exit 1; fi
echo "PASS: Got 200"

# Stop Redis
echo ">>> Stopping Redis (Simulating Failure)..."
docker stop sushi-redis
sleep 1

# Fail Open Verification
echo ">>> Verifying Fail Open (fault_tolerant=true)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/rl-test)
if [ "$STATUS" != "200" ]; then echo "FAIL: Expected 200, got $STATUS"; exit 1; fi
echo "PASS: Got 200 (Fail Open)"

# Start Redis
echo ">>> Starting Redis..."
docker start sushi-redis
sleep 2

# Update Route with fault_tolerant=false
echo ">>> Updating Route (fault_tolerant=false)..."
curl -s -X POST http://localhost:8081/services/rl-test/routes -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{
  "name": "rl-route", "paths": ["/rl-test"],
  "plugins": [{
    "name": "rate-limiting",
    "config": { "second": 5, "fault_tolerant": false }
  }]
}' > /dev/null

sleep 1

# Stop Redis
echo ">>> Stopping Redis (Simulating Failure)..."
docker stop sushi-redis
sleep 1

# Fail Closed Verification (Expect 500)
echo ">>> Verifying Fail Closed (fault_tolerant=false)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/rl-test)
# The proxy returns 500 on internal errors (like Redis failure when not fault tolerant)
if [ "$STATUS" != "500" ]; then echo "FAIL: Expected 500, got $STATUS"; exit 1; fi
echo "PASS: Got 500 (Fail Closed)"

# Start Redis
echo ">>> Starting Redis..."
docker start sushi-redis
sleep 2

# ==============================================================================
# TEST 2: HIDE CLIENT HEADERS
# ==============================================================================

# Add Route with hide_client_headers=true
echo ">>> Updating Route (hide_client_headers=true)..."
curl -s -X POST http://localhost:8081/services/rl-test/routes -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{
  "name": "rl-route", "paths": ["/rl-test"],
  "plugins": [{
    "name": "rate-limiting",
    "config": { "second": 100, "hide_client_headers": true, "fault_tolerant": true }
  }]
}' > /dev/null

sleep 1

# Verify Headers HIDDEN
echo ">>> Verifying Headers Hidden..."
HEADERS=$(curl -s -I http://localhost:8080/rl-test)
if echo "$HEADERS" | grep -q "X-RateLimit-Limit"; then
  echo "FAIL: Found X-RateLimit-Limit header (Should be hidden)"
  echo "$HEADERS"
  exit 1
fi
echo "PASS: Headers Hidden"

# Update Route with hide_client_headers=false
echo ">>> Updating Route (hide_client_headers=false)..."
curl -s -X POST http://localhost:8081/services/rl-test/routes -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{
  "name": "rl-route", "paths": ["/rl-test"],
  "plugins": [{
    "name": "rate-limiting",
    "config": { "second": 100, "hide_client_headers": false, "fault_tolerant": true }
  }]
}' > /dev/null

sleep 1

# Verify Headers VISIBLE
echo ">>> Verifying Headers Visible..."
HEADERS=$(curl -s -I http://localhost:8080/rl-test)
if ! echo "$HEADERS" | grep -q "X-RateLimit-Limit"; then
  echo "FAIL: Missing X-RateLimit-Limit header (Should be visible)"
  echo "$HEADERS"
  exit 1
fi
echo "PASS: Headers Visible"

echo "ALL ADVANCED RATELIMIT TESTS PASSED"
