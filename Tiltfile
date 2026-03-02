# Gateway-arr Tiltfile for local development

# Use GHCR
default_registry('ghcr.io/thebranchdriftcatalyst')

# Build the Go binary
local_resource(
    'gateway-arr-compile',
    'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager ./cmd/manager/main.go',
    deps=['cmd/', 'api/', 'internal/', 'go.mod', 'go.sum'],
)

# Build Docker image with live update
docker_build(
    'gateway-arr',
    '.',
    dockerfile='Dockerfile',
    live_update=[
        sync('./bin/manager', '/manager'),
    ],
)

# Apply CRD first (cluster-scoped)
k8s_yaml('config/crd/bases/gateway.catalyst.io_widgets.yaml')

# Apply RBAC
k8s_yaml('config/rbac/role.yaml')

# Apply deployment resources
k8s_yaml(kustomize('config/deploy'))

# Resource configuration
k8s_resource(
    'gateway-arr-controller',
    port_forwards=[
        port_forward(8082, 8082, name='API'),
        port_forward(8080, 8080, name='Metrics'),
    ],
    resource_deps=['gateway-arr-compile'],
    labels=['operator'],
)

# Watch sample widgets for easy testing
k8s_yaml('config/samples/widget_sonarr.yaml')
