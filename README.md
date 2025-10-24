# SQLite Operator

A Kubernetes operator for managing SQLite databases with Litestream replication in **sidecar mode**. Built with Go and the Operator SDK for high performance and reliability.

## Features

- **Sidecar Mode**: Users mount the SQLite volume directly in their application pods
- **ReadWriteMany Support**: Compatible with distributed filesystems like JuiceFS
- **Litestream Replication**: Automatic backup and replication to multiple storage backends:
  - Amazon S3
  - Azure Blob Storage
  - Google Cloud Storage
  - Local storage
- **Optional REST API**: Expose SQLite databases via RESTful API using sqlite-rest (disabled by default)
- **Authentication**: JWT-based authentication for API access
- **Ingress Support**: External access with TLS termination
- **Monitoring**: Prometheus metrics and health checks
- **High Performance**: Go-based implementation for better performance and reliability
- **Type Safety**: Compile-time validation and better error handling

## Quick Start

### 1. Deploy the Operator

```bash
# Build and push the operator image
make docker-build docker-push IMG=your-registry/sqlite-operator:v0.1.0

# Deploy the operator
make deploy IMG=your-registry/sqlite-operator:v0.1.0
```

### 2. Create a SQLite Database

```yaml
apiVersion: database.sqlite.io/v1alpha1
kind: SqliteDatabase
metadata:
  name: my-database
  namespace: default
spec:
  database:
    name: "app.db"
    storage:
      size: "2Gi"
  litestream:
    enabled: true
    replicas:
      - type: s3
        bucket: "my-backup-bucket"
        region: "us-west-2"
        path: "sqlite-backups"
        credentials:
          secretName: "s3-credentials"
  sqliteRest:
    enabled: true
    port: 8080
    authSecret: "jwt-secret"
    allowedTables: ["users", "products"]
  ingress:
    enabled: true
    host: "api.example.com"
    tls:
      enabled: true
      secretName: "api-tls"
```

### 3. Access the API

```bash
# Get the service endpoint
kubectl get svc my-database-service

# Test the API
curl -H "Authorization: Bearer $JWT_TOKEN" \
  http://my-database-service.default.svc.cluster.local:8080/users
```

## Sidecar Mode Usage

The SQLite operator runs in **sidecar mode** by default, where users mount the SQLite volume directly in their application pods. This provides better performance and flexibility compared to REST API access.

### How It Works

1. **Operator creates a PVC** with ReadWriteMany access mode (compatible with JuiceFS)
2. **Litestream pod** runs for S3 replication and backup
3. **Init container** creates an empty database if needed
4. **User applications** mount the same PVC to access SQLite directly

### Example: User Application Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1  # Important: Only 1 writer for SQLite safety
  template:
    spec:
      containers:
      - name: app
        image: my-app:latest
        env:
        - name: DATABASE_PATH
          value: "/data/app.db"
        volumeMounts:
        - name: sqlite-data
          mountPath: /data
      volumes:
      - name: sqlite-data
        persistentVolumeClaim:
          claimName: sqlitedatabase-sample-db-storage  # References the PVC created by the operator
```

### Safety Considerations

- **Single Writer**: Only run 1 replica of applications that write to SQLite
- **Multiple Readers**: You can safely run multiple read-only applications
- **ReadWriteMany**: Use distributed filesystems like JuiceFS for multi-pod access
- **Backup**: Litestream provides continuous backup to S3

### When to Enable REST API

Enable `sqliteRest` only when you need:
- External API access to your database
- Multiple applications accessing the same database via HTTP
- Integration with existing REST-based systems

## Configuration

### Database Configuration

```yaml
spec:
  database:
    name: "database.db"              # SQLite database filename
    initScript: "init-script-cm"      # ConfigMap with SQL initialization
    storage:
      size: "1Gi"                     # PVC size
      storageClass: "fast-ssd"        # Storage class (optional)
```

### Litestream Configuration

#### S3 Backend
```yaml
spec:
  litestream:
    enabled: true
    replicas:
      - type: s3
        bucket: "my-backup-bucket"
        region: "us-west-2"
        path: "sqlite-backups"
        credentials:
          secretName: "s3-credentials"
          accessKeyField: "access-key"
          secretKeyField: "secret-key"
        retention: "168h"             # 7 days
        retentionCheckInterval: "1h"
```

#### Azure Blob Storage
```yaml
spec:
  litestream:
    enabled: true
    replicas:
      - type: azure
        bucket: "mystorageaccount"     # Azure storage account
        path: "sqlite-backups"
        credentials:
          secretName: "azure-credentials"
          accessKeyField: "account-name"
          secretKeyField: "account-key"
```

#### Google Cloud Storage
```yaml
spec:
  litestream:
    enabled: true
    replicas:
      - type: gcs
        bucket: "my-backup-bucket"
        path: "sqlite-backups"
        credentials:
          secretName: "gcs-credentials"
```

### sqlite-rest Configuration

```yaml
spec:
  sqliteRest:
    enabled: true
    port: 8080                        # API port
    authSecret: "jwt-secret"          # JWT token secret
    allowedTables: ["users", "products"]  # Allowed tables
    metrics:
      enabled: true
      port: 8081                      # Metrics port
```

### Ingress Configuration

```yaml
spec:
  ingress:
    enabled: true
    host: "api.example.com"
    tls:
      enabled: true
      secretName: "api-tls"
```

## API Usage

### Authentication

The operator uses JWT tokens for authentication. Create a secret with your token:

```bash
# Generate a JWT token (example)
kubectl create secret generic jwt-secret \
  --from-literal=token="your-jwt-token"
```

### API Endpoints

- `GET /{table}` - List records
- `GET /{table}?id=eq.1` - Get specific record
- `POST /{table}` - Create record
- `PATCH /{table}?id=eq.1` - Update record
- `DELETE /{table}?id=eq.1` - Delete record

### Example Queries

```bash
# List all users
curl -H "Authorization: Bearer $JWT_TOKEN" \
  http://api.example.com/users

# Get user by ID
curl -H "Authorization: Bearer $JWT_TOKEN" \
  http://api.example.com/users?id=eq.1

# Create a new user
curl -X POST -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"john","email":"john@example.com"}' \
  http://api.example.com/users
```

## Monitoring

The operator exposes Prometheus metrics on the metrics port (default: 8081):

- `sqlite_rest_requests_total` - Total API requests
- `sqlite_rest_request_duration_seconds` - Request duration
- `litestream_replicas_total` - Number of active replicas
- `litestream_backup_duration_seconds` - Backup duration

## Development

### Prerequisites

- Go 1.21+
- Operator SDK v1.41.1+
- Docker
- kubectl
- Access to a Kubernetes cluster

### Building

```bash
# Install dependencies
go mod download

# Generate code
make generate

# Build the operator
make build

# Run tests
make test

# Build Docker image
make docker-build
```

### Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Deploy to cluster
make deploy

# Create sample database
kubectl apply -f config/samples/database_v1alpha1_sqlitedatabase.yaml
```

### Project Structure

```
├── api/v1alpha1/           # API type definitions
├── cmd/                    # Main application entry point
├── internal/controller/     # Controller logic
├── config/                 # Kubernetes manifests
│   ├── crd/               # Custom Resource Definitions
│   ├── manager/           # Manager deployment
│   └── samples/           # Example resources
├── Dockerfile             # Container image
└── Makefile              # Build automation
```

## Migration from Ansible

This operator has been migrated from Ansible-based to Go-based implementation. Key improvements:

1. **Performance**: ~10x faster reconciliation
2. **Type Safety**: Compile-time validation
3. **Smaller Image**: ~50MB vs ~500MB
4. **Better Testing**: Unit tests with mocking
5. **Standard Patterns**: Kubernetes conditions and status

The API remains backward compatible with the previous Ansible version.

## License

MIT License