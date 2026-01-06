# Sushi-Manager UI Guide

The Sushi-Manager is a modern web UI for monitoring and managing your Sushi Gateway instance. Built with React, TypeScript, and Tailwind CSS.

## Features

### 📊 Dashboard (`/`)
- Gateway configuration overview
- Environment settings
- **Rate limiting statistics** with auto-refresh

### 🔀 Services (`/services`)
- Service list with base paths and protocols
- **Upstream count** per service
- **Aggregated health status** (All Healthy / Degraded / Unhealthy)
- Load balancing algorithm display

### 🛣️ Routes (`/routes`)
- Route configuration by service
- HTTP methods and paths
- Plugin associations

### ⚡ Upstreams (`/upstreams`)
- Real-time upstream metrics
- **Health status badges** with pulse animations
- **Active connections** count
- **EWMA latency** (ms)
- **Weight** configuration
- Click row for detailed modal view

### 🏥 Health Dashboard (`/health`)
- **Auto-refreshing grid** (updates every 5s)
- Upstreams grouped by service
- Check type indicators (Active / Passive / Both)
- Success/failure statistics
- Relative timestamps ("5s ago")

### 👥 Consumers (`/consumers`)
- Consumer list with credentials
- Username, Custom ID, Created date
- Empty state for no consumers

### 🔌 Plugins (`/plugins`)
- Active plugins overview

---

## Getting Started

### Installation

```bash
cd sushi-manager
npm install
```

### Development

```bash
npm run dev
```

The UI will be available at `http://localhost:5173`

### Build for Production

```bash
npm run build
npm run preview
```

---

## Configuration

### API Endpoints

The UI fetches data from the Sushi Gateway Admin API (default: `http://localhost:8001`):

| Endpoint | Data | Refresh Rate |
|----------|------|--------------|
| `/api/gateway` | Configuration | Manual |
| `/api/health` | Health status | 5s (auto) |
| `/api/stats` | Load metrics | 10s (auto) |

### CORS Configuration

Ensure your `config.yaml` allows the UI origin:

```yaml
admin_cors_origin: "http://localhost:5173"
```

---

## Page Details

### Upstreams Page

**Route**: `/upstreams`

**Columns**:
- Service - Parent service name
- Target - Host:port
- Health - Color-coded badge (🟢 Healthy, 🔴 Unhealthy, ⚪ Unknown)
- Weight - Load balancing weight (1-1000)
- Active Conns - Current connections
- Latency (ms) - EWMA latency

**Modal**: Click any row to see full upstream details including ID.

### Health Dashboard

**Route**: `/health`

**Summary Cards**:
- Total Upstreams
- Healthy Count (green)
- Unhealthy Count (red)

**Grid Features**:
- Grouped by service name
- Status indicator with pulse animation for healthy upstreams
- Check type badges (Active, Passive, Both)
- Success (✓) and Failure (✗) counts
- Relative time since last check

**Auto-Refresh**: Updates every 5 seconds from `/api/health`

### Services Page

**Route**: `/services`

**Enhanced Columns**:
- Name
- Base Path
- Protocol
- Load Balancing - Algorithm name
- **Upstreams** - Count of upstreams
- **Health** - Aggregated status badge

**Health Aggregation**:
- 100% healthy → "All Healthy" (green with pulse)
- 50-99% healthy → "Degraded" (yellow)
- <50% healthy → "Unhealthy" (red)

---

## Design System

### Color Palette

**Health Status**:
- 🟢 Green (`bg-green-500`) - Healthy
- 🔴 Red (`bg-red-500`) - Unhealthy
- 🟡 Yellow (`bg-yellow-500`) - Degraded
- ⚪ Gray (`bg-gray-400`) - Unknown

**Check Types**:
- 🔵 Blue - Active
- 🟣 Purple - Passive
- 🟦 Indigo - Both

### Components

All components use consistent styling:
- **Cards**: `DashboardCard` with shadow and border
- **Tables**: Hover effects with `hover:bg-gray-100`
- **Badges**: Rounded pills with `px-3 py-1`
- **Modals**: Centered overlay with backdrop

---

## Development

### Project Structure

```
sushi-manager/
├── src/
│   ├── modules/
│   │   ├── upstreams/
│   │   │   ├── index.tsx           # Main page
│   │   │   ├── UpstreamTable.tsx   # Table component
│   │   │   ├── UpstreamModal.tsx   # Detail modal
│   │   │   └── HealthBadge.tsx     # Status badge
│   │   ├── health/
│   │   │   ├── index.tsx           # Health dashboard
│   │   │   └── HealthGrid.tsx      # Grid component
│   │   ├── consumers/
│   │   └── ...
│   ├── components/
│   │   ├── stats/
│   │   │   └── RateLimitCard.tsx   # Rate limit widget
│   │   └── ...
│   └── router.tsx                  # Route definitions
```

### Adding New Pages

1. Create module directory in `src/modules/`
2. Add route in `src/router.tsx`
3. Follow existing component patterns
4. Use TypeScript interfaces for data types

---

## API Integration

### Fetch Pattern

```tsx
const [data, setData] = useState([]);

useEffect(() => {
  fetchData();
  const interval = setInterval(fetchData, 5000);
  return () => clearInterval(interval);
}, []);

async function fetchData() {
  try {
    const response = await fetch("http://localhost:8001/api/stats");
    const json = await response.json();
    setData(json.upstreams || []);
  } catch (error) {
    console.error("Failed to fetch:", error);
  }
}
```

### Data Types

```tsx
interface UpstreamData {
  upstream_id: string;
  service_name: string;
  target: string;
  weight: number;
  active_connections: number;
  ewma_latency_ms: number;
  health_status: string;
}
```

---

## Troubleshooting

### CORS Errors

**Symptom**: Network errors in browser console

**Solution**: Update `config.yaml`:
```yaml
admin_cors_origin: "http://localhost:5173"
```

### Empty Data

**Symptom**: "No data available" messages

**Causes**:
- Gateway not running
- Wrong API URL
- No services configured

**Solution**: Check API endpoints:
```bash
curl http://localhost:8001/api/health
curl http://localhost:8001/api/stats
```

### Stale Data

**Symptom**: Data not updating

**Cause**: Auto-refresh not working

**Solution**: Check browser console for errors, verify API accessibility

---

## Performance

### Optimization Features

- **React Hooks**: Efficient state management
- **Auto-Refresh**: Only affected pages poll APIs
- **Loading States**: Prevents layout shift
- **Empty States**: Graceful handling of no data

### Recommended Refresh Rates

- Health Dashboard: 5 seconds
- Stats/Metrics: 10 seconds
- Configuration: Manual (30s+ if needed)

---

## Browser Compatibility

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

---

## Next Steps

- [Admin API Reference](../api/admin-api.md)
- [Load Balancing Guide](../concepts/load-balancing.md)
- [Health Checks](../concepts/health-checks.md)
