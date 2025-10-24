# SQLite Operator Deployment Guide

## Current Status
✅ **Operator Built Successfully** - Docker image `sqlite-operator:v0.1.0` is ready
⚠️ **Deployment Issue** - Kubernetes can't pull the local image

## Solutions

### Option 1: Push to Container Registry (Recommended)

1. **Tag and push to a registry:**
```bash
# Tag for your registry
docker tag sqlite-operator:v0.1.0 your-registry.com/sqlite-operator:v0.1.0

# Push to registry
docker push your-registry.com/sqlite-operator:v0.1.0

# Update deployment
kubectl patch deployment sqlite-operator-controller-manager -n sqlite-operator-system \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","image":"your-registry.com/sqlite-operator:v0.1.0"}]}}}}'
```

### Option 2: Load Image to Cluster (if using kind/minikube)

```bash
# For kind
kind load docker-image sqlite-operator:v0.1.0

# For minikube
minikube image load sqlite-operator:v0.1.0
```

### Option 3: Run Operator Locally (Development)

```bash
# Set up kubeconfig
export KUBECONFIG=~/.kube/config

# Run operator locally
cd /home/linux/projects/sqlite-operator
ansible-operator run
```

## Testing with Wasabi Storage

### 1. Create Wasabi Credentials

```bash
# Create base64 encoded credentials
echo -n "your-wasabi-access-key" | base64
echo -n "your-wasabi-secret-key" | base64

# Update the secret in wasabi-credentials.yaml with your encoded values
kubectl apply -f examples/wasabi-credentials.yaml
```

### 2. Deploy SQLite Database

```bash
# Deploy the database with Wasabi backup
kubectl apply -f examples/wasabi-backup.yaml
```

### 3. Verify Deployment

```bash
# Check if database is running
kubectl get sqlitedatabases

# Check pods
kubectl get pods -l app.kubernetes.io/name=sqlite-database

# Check services
kubectl get svc -l app.kubernetes.io/name=sqlite-database
```

### 4. Test the API

```bash
# Port forward to access the API
kubectl port-forward svc/app-with-wasabi-backup-service 8080:8080

# Test API (in another terminal)
curl -H "Authorization: Bearer your-jwt-token" \
  http://localhost:8080/users
```

## Wasabi Configuration

The operator is configured to use Wasabi with:
- **Storage Class**: `sc-wasabi-us-east-1`
- **Region**: `us-east-1`
- **Endpoint**: Automatically detected as S3-compatible
- **Retention**: 30 days with 6-hour check intervals

## Troubleshooting

### Check Operator Logs
```bash
kubectl logs -n sqlite-operator-system deployment/sqlite-operator-controller-manager
```

### Check Database Status
```bash
kubectl describe sqlitedatabase app-with-wasabi-backup
```

### Check Litestream Logs
```bash
kubectl logs -l app.kubernetes.io/name=sqlite-database -c litestream
```

## Next Steps

1. **Fix Image Pull Issue**: Use Option 1 (push to registry) for production
2. **Configure Wasabi**: Update credentials in `wasabi-credentials.yaml`
3. **Test Backup**: Verify Litestream is replicating to Wasabi
4. **Monitor**: Check metrics endpoint at `:8081/metrics`

## Production Considerations

- Use proper image registry (Docker Hub, ECR, GCR, etc.)
- Set up proper RBAC and security policies
- Configure resource limits and requests
- Set up monitoring and alerting
- Use proper TLS certificates for Ingress
- Configure backup retention policies
