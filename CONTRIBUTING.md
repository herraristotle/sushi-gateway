# Contributing to Sushi Gateway

Thank you for your interest in contributing to Sushi Gateway! This guide will help you get started with development.

## 🛠️ Development Environment Setup

### Prerequisites

- **Go 1.23+** - [Install Go](https://go.dev/doc/install)
- **Docker & Docker Compose** - For running Redis and E2E tests
- **Make** (optional) - For simplified commands
- **Node.js 18+** (optional) - For sushi-manager UI development

### Clone the Repository

```bash
git clone https://github.com/rawsashimi1604/sushi-gateway.git
cd sushi-gateway
```

### Project Structure

```
sushi-gateway/
├── sushi-proxy/          # Core Gateway (Go)
│   ├── cmd/              # Main entrypoint
│   ├── config/           # Configuration files
│   ├── internal/
│   │   ├── api/          # Admin API endpoints
│   │   ├── gateway/      # Plugins, proxy logic
│   │   ├── container/    # Dependency injection
│   │   ├── discovery/    # Service discovery
│   │   └── model/        # Data models
│   └── go.mod
├── sushi-manager/        # Web UI (React/TypeScript)
├── docs/                 # VitePress documentation
└── e2e-tests/            # End-to-end tests
```

## 🚀 Running the Gateway

### 1. Start Redis

```bash
docker run -d --name sushi-redis -p 6379:6379 redis:alpine
```

### 2. Configure Environment

```bash
cd sushi-proxy
cp .env.example .env

# Edit .env with your settings:
# ADMIN_USER=admin
# ADMIN_PASSWORD=secret
# REDIS_ADDR=localhost:6379
# CONFIG_FILE_PATH=config/config.yaml
```

### 3. Build and Run

```bash
# Build
go build -o sushi-proxy ./cmd

# Run
./sushi-proxy
```

The gateway will be available at:
- HTTP Proxy: `http://localhost:8080`
- HTTPS Proxy: `https://localhost:8443`
- Admin API: `http://localhost:8081`

## 🧪 Running Tests

### Unit Tests

```bash
cd sushi-proxy
go test ./... -v
```

### E2E Tests

```bash
# From project root
docker compose -f docker-compose.e2e.yml up --build

# View results
docker compose -f docker-compose.e2e.yml logs e2e-tests
```

## 📝 Creating a New Plugin

Plugins are the core extension mechanism. Here's how to create one:

### 1. Create the Plugin File

```bash
touch sushi-proxy/internal/gateway/my_plugin.go
```

### 2. Implement the Plugin

```go
package gateway

import (
    "net/http"
    "github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
)

type MyPlugin struct {
    config map[string]interface{}
}

func NewMyPlugin(config map[string]interface{}) *Plugin {
    return &Plugin{
        Name:     constant.PLUGIN_MY_PLUGIN,
        Priority: 500,  // Higher = runs earlier
        Handler:  &MyPlugin{config: config},
        Validator: &MyPlugin{config: config},
    }
}

func (p *MyPlugin) Validate() error {
    // Validate configuration
    return nil
}

func (p *MyPlugin) Execute(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Your plugin logic here
        
        // Continue to next plugin
        next.ServeHTTP(w, r)
    })
}
```

### 3. Register the Plugin

Add to `internal/constant/constant.go`:
```go
const PLUGIN_MY_PLUGIN = "my_plugin"
```

Add to `AVAILABLE_PLUGINS` list and `plugin_manager.go`.

### 4. Write Tests

```bash
touch sushi-proxy/internal/gateway/my_plugin_test.go
```

## 🔀 Pull Request Process

1. **Fork** the repository
2. **Create a branch**: `git checkout -b feature/my-feature`
3. **Make changes** and add tests
4. **Run tests**: `go test ./...`
5. **Commit**: `git commit -m "feat: add my feature"`
6. **Push**: `git push origin feature/my-feature`
7. **Open PR** against `main` branch

### Commit Message Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation
- `refactor:` - Code refactoring
- `test:` - Adding tests
- `chore:` - Maintenance

## 📊 Performance Benchmarking

Run benchmarks to measure gateway performance:

```bash
cd sushi-proxy
./scripts/benchmark.sh
```

See [Performance Report](./BENCHMARK.md) for results.

## 🤝 Getting Help

- **Discord**: [Join our Discord](https://discord.gg/aPv4QhQ6)
- **Discussions**: [GitHub Discussions](https://github.com/rawsashimi1604/sushi-gateway/discussions)
- **Issues**: [GitHub Issues](https://github.com/rawsashimi1604/sushi-gateway/issues)

## 📄 License

By contributing, you agree that your contributions will be licensed under the MIT License.
