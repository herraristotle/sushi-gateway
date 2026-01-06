# Sushi Manager

Web-based UI for monitoring and managing Sushi Gateway.

## Features

| Page | Description |
|------|-------------|
| `/` | Dashboard with gateway config and rate limiting stats |
| `/services` | Service list with upstream count and health status |
| `/routes` | Route configuration by service |
| `/plugins` | Active plugins overview |
| `/upstreams` | **Real-time upstream metrics** (connections, latency, weights) |
| `/health` | **Health dashboard** with auto-refresh (5s) |
| `/consumers` | Consumer management |

### Real-time Features

- **Health Badges**: Color-coded (green/yellow/red) with pulse animations
- **Auto-Refresh**: Health dashboard updates every 5 seconds
- **Rate Limit Stats**: Hits/allowed/rate metrics on dashboard

## Development

Ensure that sushi gateway is running with the Admin API up.

### Requirements

- Node.js 18+
- npm or yarn
- Sushi Gateway running on port 8001 (Admin API)

### Create env file

Create `.env` file following the `.env.example` contents. Put it in the root directory

### Starting the server

```bash
npm install
npm run dev
```

The UI will be available at `http://localhost:5173`

### Building the source code

```bash
npm run build
```

### Docker build

```bash
docker build -t sushi-manager:latest .
```

### Docker Run

```bash
docker run --rm -p 5173:5173 \
-e SUSHI_MANAGER_BACKEND_API_URL=http://localhost:8081 \
sushi-manager:latest
```

## API Integration

The UI fetches data from these Admin API endpoints:

| Endpoint | Purpose | Refresh |
|----------|---------|---------|
| `/api/gateway` | Configuration | Manual |
| `/api/health` | Upstream health | 5s auto |
| `/api/stats` | Load balancing metrics | 10s auto |

## Project Structure

```
src/
├── modules/
│   ├── index/          # Dashboard
│   ├── services/       # Services page
│   ├── routes/         # Routes page
│   ├── plugins/        # Plugins page
│   ├── upstreams/      # Upstreams monitoring
│   ├── health/         # Health dashboard
│   └── consumers/      # Consumer management
├── components/
│   ├── layout/         # Layout components
│   ├── typography/     # Text components
│   └── stats/          # Stats widgets
└── router.tsx          # Route definitions
```

## Configuration

Update `admin_cors_origin` in your gateway config to allow UI access:

```yaml
admin_cors_origin: "http://localhost:5173"
```
