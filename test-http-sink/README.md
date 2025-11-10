# Test HTTP Sink - Mock Ingestion Edge

Minimal HTTP server to test throughput of HTTP telemetry submissions from GKE.

## What This Server Does

**Accepts:** `POST /submit/{namespace}/{docType}/{version}/{id}` (matches ingestion-edge format)
**Returns:** 200 OK immediately (no processing)
**Logs:** Request count and throughput every 10 seconds
**Exposes:**
- `/metrics` - Prometheus metrics
- `/__heartbeat__` - Health check
- `/__lbheartbeat__` - Load balancer health check

## Deployment

### 1. Build and Push

```bash
cd test-http-sink

# Build for GKE
docker build --platform linux/amd64 -t test-http-sink .

# Tag and push to GCR
docker tag test-http-sink gcr.io/akomar-server-telemetry-poc/test-http-sink
docker push gcr.io/akomar-server-telemetry-poc/test-http-sink
```

### 2. Deploy to GKE

```bash
kubectl apply -f ../kubernetes/test-http-sink-deploy.yaml

# Wait for LoadBalancer to get external IP
kubectl get service test-http-sink -w
```

**Note:** LoadBalancer provisioning takes 2-3 minutes.

### 3. Get External IP

```bash
# Get the external IP address
SINK_IP=$(kubectl get service test-http-sink -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
echo "Mock sink endpoint: http://${SINK_IP}"
```

## Run Tests Against Mock Sink

### Update Publisher Configuration

```bash
# Update ConfigMap to point to mock sink
kubectl patch configmap test-go-http-publisher-config -p "{\"data\":{\"ENDPOINT\":\"http://${SINK_IP}/submit\"}}"

# Restart publisher to pick up new endpoint
kubectl rollout restart deployment/test-go-http-publisher
```

### Run Tests

```bash
kubectl patch configmap test-go-http-publisher-config -p '{"data":{"EVENTS_PER_SEC":"20000","CONCURRENCY":"10"}}'
kubectl rollout restart deployment/test-go-http-publisher
```

## Monitoring

### Publisher Side (Client)

```bash
kubectl logs -f deployment/test-go-http-publisher
```

### Sink Side (Server)

```bash
kubectl logs -f deployment/test-http-sink
```
