# Development Guide

This guide covers development setup, testing, and contribution guidelines for the SQLite Operator.

## Prerequisites

- Go 1.21 or later
- Docker
- kubectl
- Operator SDK v1.41.1+
- Access to a Kubernetes cluster (kind, minikube, or cloud)

## Development Setup

### 1. Clone and Setup

```bash
git clone <repository-url>
cd sqlite-operator
go mod download
```

### 2. Install Dependencies

```bash
# Install Operator SDK
curl -LO https://github.com/operator-framework/operator-sdk/releases/download/v1.41.1/operator-sdk_linux_amd64
chmod +x operator-sdk_linux_amd64
sudo mv operator-sdk_linux_amd64 /usr/local/bin/operator-sdk

# Install controller-gen
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
```

### 3. Code Generation

```bash
# Generate deepcopy methods
make generate

# Generate manifests (CRDs, RBAC)
make manifests
```

## Development Workflow

### 1. Making Changes

1. **API Changes**: Modify types in `api/v1alpha1/sqlitedatabase_types.go`
2. **Controller Logic**: Update `internal/controller/sqlitedatabase_controller.go`
3. **Tests**: Add/update tests in `internal/controller/sqlitedatabase_controller_test.go`

### 2. Testing Changes

```bash
# Run unit tests
make test

# Run with coverage
make test COVERAGE=1

# Run integration tests
make test-integration

# Test with real cluster
make deploy
kubectl apply -f config/samples/database_v1alpha1_sqlitedatabase.yaml
```

### 3. Building and Deploying

```bash
# Build locally
make build

# Build Docker image
make docker-build IMG=your-registry/sqlite-operator:dev

# Deploy to cluster
make deploy IMG=your-registry/sqlite-operator:dev
```

## Architecture

### Controller Logic

The controller follows the standard Kubernetes operator pattern:

1. **Reconcile Loop**: Main reconciliation logic in `Reconcile()` method
2. **Resource Management**: Create/update Kubernetes resources
3. **Status Updates**: Update CR status with conditions
4. **Error Handling**: Proper error handling and retries

### Key Components

- **API Types**: Defined in `api/v1alpha1/`
- **Controller**: Main logic in `internal/controller/`
- **Manifests**: Kubernetes resources in `config/`
- **Tests**: Unit and integration tests

### Resource Flow

```
SqliteDatabase CR
    ↓
Controller Reconcile
    ↓
Create/Update Resources:
- PVC (database storage)
- ConfigMaps (litestream, sqlite-rest config)
- Deployment (sqlite + litestream + sqlite-rest)
- Service (REST API)
- Ingress (optional)
    ↓
Update Status & Conditions
```

## Testing

### Unit Tests

Located in `internal/controller/sqlitedatabase_controller_test.go`:

```bash
# Run unit tests
make test

# Run specific test
go test -run TestSqliteDatabase ./internal/controller/
```

### Integration Tests

Test with real Kubernetes cluster:

```bash
# Start kind cluster
kind create cluster

# Deploy operator
make deploy

# Apply sample CR
kubectl apply -f config/samples/database_v1alpha1_sqlitedatabase.yaml

# Check status
kubectl get sqlitedatabases
kubectl describe sqlitedatabase sqlitedatabase-sample
```

### Test Coverage

```bash
# Generate coverage report
make test COVERAGE=1
go tool cover -html=cover.out
```

## Debugging

### Local Development

```bash
# Run controller locally
make run

# Debug with Delve
dlv debug ./cmd/main.go
```

### Cluster Debugging

```bash
# Check controller logs
kubectl logs -n system deployment/controller-manager

# Check CR status
kubectl get sqlitedatabases -o yaml

# Check created resources
kubectl get all -l app.kubernetes.io/name=sqlite-database
```

## Performance Considerations

### Resource Limits

- **CPU**: 10m request, 500m limit
- **Memory**: 64Mi request, 128Mi limit
- **Storage**: Configurable via spec

### Optimization Tips

1. **Batch Operations**: Use `CreateOrUpdate` for efficiency
2. **Status Updates**: Minimize status update frequency
3. **Resource Cleanup**: Proper owner references
4. **Error Handling**: Exponential backoff for retries

## Contributing

### Code Style

- Follow Go conventions
- Use `gofmt` and `golint`
- Add tests for new features
- Update documentation

### Pull Request Process

1. Fork the repository
2. Create feature branch
3. Make changes with tests
4. Run `make test` and `make lint`
5. Submit pull request

### Commit Messages

Use conventional commits:

```
feat: add new feature
fix: bug fix
docs: documentation update
test: add tests
refactor: code refactoring
```

## Troubleshooting

### Common Issues

1. **CRD Not Found**: Run `make install`
2. **RBAC Errors**: Check cluster permissions
3. **Image Pull Errors**: Build and push image
4. **Reconciliation Loops**: Check controller logs

### Debug Commands

```bash
# Check CRD installation
kubectl get crd sqlitedatabases.database.sqlite.io

# Check controller status
kubectl get deployment -n system controller-manager

# Check CR status
kubectl get sqlitedatabases -o wide

# View controller logs
kubectl logs -n system deployment/controller-manager -f
```

## Release Process

### Versioning

- Follow semantic versioning (v1.2.3)
- Update VERSION in Makefile
- Tag releases in git

### Release Steps

```bash
# Update version
export VERSION=v1.0.0

# Build and push
make docker-build docker-push IMG=your-registry/sqlite-operator:$VERSION

# Generate bundle
make bundle

# Build bundle image
make bundle-build BUNDLE_IMG=your-registry/sqlite-operator-bundle:$VERSION
```

## Resources

- [Operator SDK Documentation](https://sdk.operatorframework.io/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [Controller Runtime](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)
