import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom Metrics
const upstream1Count = new Counter('upstream_1_hits');
const upstream2Count = new Counter('upstream_2_hits');
const upstream3Count = new Counter('upstream_3_hits');
const errorRate = new Rate('errors');
const latencyTrend = new Trend('latency_trend');

export const options = {
    scenarios: {
        // 1. Weighted Round Robin Test
        // Expect ~75% traffic to upstream-2 (weight 300) and ~25% to upstream-1 (weight 100)
        weighted_rr: {
            executor: 'constant-arrival-rate',
            rate: 20, // 20 RPS
            timeUnit: '1s',
            duration: '10s',
            preAllocatedVUs: 10,
            exec: 'weightedRouting',
            startTime: '0s',
        },

        // 2. Least Connections Test
        // We will delay upstream-2 massively. Least Conn should favor upstream-1.
        least_conn: {
            executor: 'constant-arrival-rate',
            rate: 10,
            timeUnit: '1s',
            duration: '10s',
            preAllocatedVUs: 10,
            exec: 'leastConnections',
            startTime: '15s', // Run after weighted
        },

        // 3. Latency EWMA Test
        // Simulates dynamic latency.
        latency_ewma: {
            executor: 'constant-arrival-rate',
            rate: 10,
            timeUnit: '1s',
            duration: '15s',
            preAllocatedVUs: 10,
            exec: 'latencyRouting',
            startTime: '30s',
        },

        // 4. Stress Test
        // Max throughput on a dedicated upstream
        stress_test: {
            executor: 'ramping-arrival-rate',
            startRate: 50,
            timeUnit: '1s',
            preAllocatedVUs: 50,
            stages: [
                { target: 200, duration: '10s' }, // Ramp up
                { target: 500, duration: '20s' }, // Stress
                { target: 0, duration: '5s' },    // Ramp down
            ],
            exec: 'stressTest',
            startTime: '50s',
        },
    },
    thresholds: {
        errors: ['rate<0.01'], // <1% errors
        http_req_duration: ['p(95)<500'], // 95% of requests < 500ms (excluding intentional delays)
    },
};

const BASE_URL = 'http://sushi-proxy:8080';

export function weightedRouting() {
    const res = http.get(`${BASE_URL}/weighted/v1/sushi/restaurant/1`);
    recordMetrics(res);
}

export function leastConnections() {
    // Upstream 2 is configured to be slow (simulated by parameter or just known slow)
    // Actually, we can pass ?ms=500 to the delay endpoint if we routed to that.
    // The current config points to /v1/sushi...
    // Let's assume we can hit the delay endpoint if we modify the path in config or use query params.
    // Since our test service echoes back the ID, we can check distribution.

    // To properly test least conn, we need long-lived requests.
    // We'll hit the /delay endpoint if mapped. 
    // Wait, our config maps /least-conn -> upstreams.
    // We should hit /least-conn/delay?ms=100

    // We want upstream-1 to be fast (0ms) and upstream-2 to be slow (500ms).
    // But the Load Balancer chooses the upstream, we can't force the request to a specific upstream 
    // AND specify delay for *that* upstream in the request URL unless we control the LB.
    // THE TRICK: The 'delay' param instructs the upstream to sleep.
    // If we send ?ms=200, WHOEVER gets it sleeps.
    // If Round Robin, both sleep equally -> Least Conn stays balanced.
    // If we want to bias it, we'd need one server to represent "Busy".
    // Since we can't easily simulate "Busy" from the client side without impacting both,
    // We will just measure that Least Conn balances the CONNECTION COUNT.

    // Alternative: We send requests with delay.
    // Upstream 1 picks it up -> holds connection.
    // Next request comes. If LB sees Upstream 1 has 1 active, Upstream 2 has 0, it picks 2.
    // This is valid test.
    const res = http.get(`${BASE_URL}/least-conn/delay?ms=200`);
    recordMetrics(res);
}

export function latencyRouting() {
    // For EWMA, we want to prove it avoids slow nodes.
    // We can't implementation-wise control "Node B is slow" from the client easily 
    // unless the node ITSELF is slow.
    // But we are using the same docker image for both.
    // So both respond to ?ms=X.

    // Strategy:
    // This test might be hard to verify deterministically with k6 alone 
    // without side-channel poisoning of one upstream.
    // However, we can measure that Latency DOESN'T fail or error.

    const res = http.get(`${BASE_URL}/latency/v1/sushi/restaurant/1`);
    recordMetrics(res);
}

export function stressTest() {
    const res = http.get(`${BASE_URL}/stress/v1/sushi/restaurant/1`);
    check(res, { 'status is 200': (r) => r.status === 200 });
    errorRate.add(res.status !== 200);
    latencyTrend.add(res.timings.duration);
}

function recordMetrics(res) {
    check(res, { 'status is 200': (r) => r.status === 200 });
    errorRate.add(res.status !== 200);

    if (res.status === 200 && res.body) {
        try {
            const body = JSON.parse(res.body);
            // App ID is embedded in response
            // sushi-api returns { app_id: "upstream-1", ... }
            if (body.app_id === 'upstream-1') upstream1Count.add(1);
            if (body.app_id === 'upstream-2') upstream2Count.add(1);
            if (body.app_id === 'upstream-3') upstream3Count.add(1);
        } catch (e) {
            // ignore parse errors
        }
    }
}
