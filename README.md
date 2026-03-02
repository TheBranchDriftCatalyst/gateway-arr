# Gateway-arr

Kubernetes operator that aggregates Widget CRDs into Homepage-compatible configuration, providing a unified navigation gateway for *arr apps and other homelab services.

## Overview

Gateway-arr watches Widget custom resources across your cluster and:
1. Generates Homepage `services.yaml` configuration automatically
2. Provides a REST API for widget data consumption
3. Supports real-time WebSocket updates
4. Handles credential injection via Kubernetes secrets

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Gateway-arr                               │
├─────────────────────────────────────────────────────────────────┤
│  Widget CRDs (sonarr, radarr, plex, etc.)                       │
│         │                                                        │
│         ▼                                                        │
│  ┌──────────────────┐    ┌──────────────────┐                   │
│  │  Go Controller   │───▶│  Homepage        │                   │
│  │  (reconciler)    │    │  ConfigMap       │                   │
│  └────────┬─────────┘    └──────────────────┘                   │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────┐                                           │
│  │  REST API        │◀── Homepage UI / Custom UIs               │
│  │  + WebSocket     │                                           │
│  └──────────────────┘                                           │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.22+
- Kubernetes cluster with kubectl configured
- (Optional) Docker for building images

### Install CRDs

```bash
make install
# or
kubectl apply -f config/crd/bases/
```

### Run Locally

```bash
# Download dependencies
make deps

# Run the operator (connects to current kubectl context)
make run
```

### Deploy Sample Widgets

```bash
kubectl apply -f config/samples/
```

### Check Generated ConfigMap

```bash
kubectl get configmap homepage-services -n homepage -o yaml
```

## Widget CRD

Example Widget resource:

```yaml
apiVersion: gateway.catalyst.io/v1alpha1
kind: Widget
metadata:
  name: sonarr
  namespace: media
  labels:
    gateway.catalyst.io/category: "Media Management"
    gateway.catalyst.io/order: "1"
spec:
  displayName: Sonarr
  description: TV Series Management
  icon: sonarr.png
  href: http://sonarr.talos00
  internalUrl: http://sonarr.media.svc.cluster.local:8989
  siteMonitor:
    enabled: true
    path: /api/health
  widget:
    type: sonarr
    enableQueue: true
    credentials:
      apiKeySecretRef:
        name: sonarr-credentials
        key: api-key
  nav:
    showInOverlay: true
    shortcut: "s"
```

### Labels

- `gateway.catalyst.io/category`: Groups widgets in Homepage categories
- `gateway.catalyst.io/order`: Sort order within category

### Spec Fields

| Field | Description |
|-------|-------------|
| `displayName` | Name shown in UI |
| `description` | Service description |
| `icon` | Icon filename or URL |
| `href` | External URL for user access |
| `internalUrl` | Cluster-internal service URL (for API calls) |
| `siteMonitor` | Health monitoring configuration |
| `widget` | Homepage widget configuration |
| `nav` | Navigation overlay settings |

## REST API

The operator exposes a REST API on port 8082:

```bash
# List all widgets
curl http://localhost:8082/api/widgets

# List grouped by category
curl http://localhost:8082/api/widgets?groupBy=category

# Get single widget
curl http://localhost:8082/api/widgets/media/sonarr

# Health check
curl http://localhost:8082/api/health
```

### WebSocket

Connect to `/ws` for real-time widget updates.

## Deployment

### Build & Push Image

```bash
make docker-build IMG=registry.talos00/gateway-arr:latest
make docker-push IMG=registry.talos00/gateway-arr:latest
```

### Deploy to Cluster

```bash
# Update image in config/manager/manager.yaml
make deploy
```

### Configuration

Operator flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--target-namespace` | `homepage` | Namespace for Homepage ConfigMap |
| `--configmap-name` | `homepage-services` | Name of services ConfigMap |
| `--api-bind-address` | `:8082` | REST API listen address |
| `--metrics-bind-address` | `:8080` | Metrics endpoint |
| `--health-probe-bind-address` | `:8081` | Health probes endpoint |

## UI

The `ui/` directory contains a fork of [Homepage](https://github.com/gethomepage/homepage) configured to consume widget data from the Gateway-arr API.

```bash
cd ui
pnpm install
pnpm dev
```

## Development

### Project Structure

```
gateway-arr/
├── api/v1alpha1/          # CRD type definitions
├── cmd/manager/           # Operator entrypoint
├── internal/
│   ├── controller/        # Reconciliation logic
│   └── server/            # REST API
├── config/
│   ├── crd/bases/         # CRD manifests
│   ├── rbac/              # RBAC configuration
│   ├── manager/           # Deployment manifests
│   └── samples/           # Example widgets
├── ui/                    # Homepage fork
├── Dockerfile
├── Makefile
└── Taskfile.yaml
```

### Make Targets

```bash
make help           # Show all targets
make deps           # Download dependencies
make build          # Build binary
make run            # Run locally
make test           # Run tests
make install        # Install CRDs
make deploy         # Deploy to cluster
make deploy-samples # Deploy sample widgets
```

## Credential Handling

Widget credentials are referenced from Kubernetes secrets. The operator generates `HOMEPAGE_VAR_` environment variable references that Homepage resolves at runtime.

Example secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sonarr-credentials
  namespace: media
type: Opaque
stringData:
  api-key: "your-api-key-here"
```

The generated Homepage config will reference this as:

```yaml
widget:
  type: sonarr
  key: "{{HOMEPAGE_VAR_MEDIA_SONARR_APIKEY}}"
```

## License

MIT
