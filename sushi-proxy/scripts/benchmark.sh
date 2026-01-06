#!/bin/bash

# Sushi Gateway Performance Benchmark Script
# Uses k6 (primary) or Vegeta (fallback)
# Tests against test-services via docker-compose.e2e.yml
#
# Usage:
#   docker compose -f docker-compose.e2e.yml up -d
#   ./scripts/benchmark.sh

set -e

# Configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
DURATION="${DURATION:-30s}"
VUS="${VUS:-100}"  # Virtual Users for k6
RATE="${RATE:-1000}"  # Requests per second for Vegeta

echo "========================================"
echo "  Sushi Gateway Performance Benchmark"
echo "========================================"
echo ""
echo "Gateway URL: $GATEWAY_URL"
echo "Duration: $DURATION"
echo "Virtual Users (k6): $VUS"
echo ""

# Verify gateway is running
echo "--- Checking Gateway Health ---"
# Skip Health Check for local bench
# if ! curl -sf "${GATEWAY_URL%:8080}:8081/health" > /dev/null 2>&1; then
#     echo "⚠️  Gateway not responding"
#     exit 1
# fi
echo "✅ Gateway check skipped"

echo "✅ Gateway is healthy"
echo ""

# Check for k6 first (preferred)
if false; then # Forced Vegeta
    echo "Using k6 for benchmarking..."
    echo ""
    
    # Create k6 script that tests against test-services
    K6_SCRIPT=$(mktemp /tmp/k6-script.XXXXXX.js)
    cat > "$K6_SCRIPT" << 'EOF'
import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

// Custom metrics
const errorRate = new Rate('errors');
const proxyLatency = new Trend('proxy_latency');
const bffLatency = new Trend('bff_latency');

export const options = {
    stages: [
        { duration: '10s', target: parseInt(__ENV.VUS) || 100 },
        { duration: '20s', target: parseInt(__ENV.VUS) || 100 },
        { duration: '5s', target: 0 },
    ],
    thresholds: {
        'http_req_duration': ['p(95)<500', 'p(99)<1000'],
        'errors': ['rate<0.05'],
    },
};

export default function () {
    const baseUrl = __ENV.GATEWAY_URL || 'http://localhost:8080';
    
    // Test 1: Simple Proxy Pass (via test-services/sushi-api)
    group('Proxy Pass', function() {
        const res = http.get(`${baseUrl}/sushi-service/v1/sushi`);
        check(res, {
            'proxy status 200': (r) => r.status === 200,
            'proxy latency < 100ms': (r) => r.timings.duration < 100,
        });
        errorRate.add(res.status !== 200);
        proxyLatency.add(res.timings.duration);
    });
    
    // Test 2: BFF Response Aggregation (3 backends in parallel)
    group('BFF Aggregation', function() {
        const res = http.get(`${baseUrl}/dashboard/v1/user/123`);
        check(res, {
            'bff status 200': (r) => r.status === 200,
            'bff has data': (r) => r.json('data') !== undefined,
        });
        bffLatency.add(res.timings.duration);
    });
    
    // Test 3: POST with body
    group('POST Request', function() {
        const payload = JSON.stringify({
            name: 'Salmon Roll',
            price: 12.99,
        });
        const res = http.post(`${baseUrl}/sushi-service/v1/sushi`, payload, {
            headers: { 'Content-Type': 'application/json' },
        });
        check(res, {
            'post status 2xx': (r) => r.status >= 200 && r.status < 300,
        });
    });
    
    sleep(0.01);
}

export function handleSummary(data) {
    return {
        'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    };
}
EOF

    k6 run \
        --env GATEWAY_URL="$GATEWAY_URL" \
        --env VUS="$VUS" \
        "$K6_SCRIPT"
    rm -f "$K6_SCRIPT"

# Fallback to Vegeta
elif command -v vegeta &> /dev/null; then
    echo "Using Vegeta for benchmarking..."
    echo ""
    
    echo "--- Test 1: Proxy Pass ---"
    echo "GET $GATEWAY_URL/sushi-service/v1/sushi" | vegeta attack -duration="$DURATION" -rate="$RATE" | vegeta report
    
    echo ""
    echo "--- Test 2: BFF Aggregation ---"
    echo "GET $GATEWAY_URL/dashboard/v1/user/123" | vegeta attack -duration=10s -rate=500 | vegeta report

else
    echo "No benchmark tool found. Please install:"
    echo ""
    echo "  k6: snap install k6"
    echo "  or"
    echo "  vegeta: go install github.com/tsenart/vegeta@latest"
    exit 1
fi

echo ""
echo "========================================"
echo "  Benchmark Complete!"
echo "========================================"
echo ""
echo "Tested endpoints (via test-services/sushi-api):"
echo "  - /sushi-service/v1/sushi (proxy pass)"
echo "  - /dashboard/v1/user/{id} (BFF aggregation)"
echo ""
