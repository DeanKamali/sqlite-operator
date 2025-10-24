# SQLite Operator

A Kubernetes operator for SQLite databases with Litestream replication in **sidecar mode**.

## Features

- **Sidecar Mode**: Mount SQLite volume directly in your application pods
- **ReadWriteMany**: Compatible with JuiceFS and distributed filesystems
- **Litestream Backup**: Automatic S3/Azure/GCS replication
- **Optional REST API**: sqlite-rest disabled by default
- **Go-based**: High performance and reliability

## Quick Start

### 1. Deploy Operator

```bash
make docker-build docker-push IMG=your-registry/sqlite-operator:v0.1.0
make deploy IMG=your-registry/sqlite-operator:v0.1.0
```

### 2. Create Database

```yaml
apiVersion: database.sqlite.io/v1alpha1
kind: SqliteDatabase
metadata:
  name: my-database
spec:
  database:
    name: "app.db"
    storage:
      size: "2Gi"
      storageClass: "juicefs"  # For ReadWriteMany
  litestream:
    enabled: true
    replicas:
      - type: s3
        bucket: "my-backup-bucket"
        credentials:
          secretName: "s3-credentials"
```

### 3. Use in Your App

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1  # Only 1 writer for SQLite safety
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
          claimName: my-database-db-storage
```

## Configuration

### Database

```yaml
spec:
  database:
    name: "database.db"
    storage:
      size: "1Gi"
      storageClass: "juicefs"
      accessMode: "ReadWriteMany"  # Default
```

### Litestream (S3)

```yaml
spec:
  litestream:
    enabled: true
    replicas:
      - type: s3
        bucket: "my-backup-bucket"
        region: "us-west-2"
        credentials:
          secretName: "s3-credentials"
        retention: "168h"
```

### Optional REST API

```yaml
spec:
  sqliteRest:
    enabled: true  # Opt-in for REST API
    port: 8080
    authSecret: "jwt-secret"
```

## Safety Notes

- **Single Writer**: Only run 1 replica of apps that write to SQLite
- **Multiple Readers**: Safe to run multiple read-only apps
- **ReadWriteMany**: Use JuiceFS or similar for multi-pod access

## Development

```bash
# Build
make build

# Test
make test

# Deploy
make deploy IMG=your-registry/sqlite-operator:v0.1.0
```

## License

Apache 2.0