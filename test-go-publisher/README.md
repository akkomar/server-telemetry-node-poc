# test-go-publisher

Direct Pub/Sub publisher for server-side Glean telemetry events (DENG-9533).

This application publishes Glean events directly to Google Cloud Pub/Sub, bypassing Cloud Logging infrastructure for improved reliability and lower latency.

## Features

- **High-throughput publishing**: Supports 40k+ msgs/s with batching
- **Asynchronous publishing**: Non-blocking with background result processing
- **Production-ready**: Graceful shutdown, error counting, stats reporting
- **Cloud Logging compatible**: Messages formatted for ParseLogEntry decoder

## Local Development

### Prerequisites

```bash
# Install dependencies
go mod download

# Setup GCP authentication
gcloud auth application-default login
```

### Running Locally

**Basic usage:**
```bash
go run main.go \
  --project=akomar-server-telemetry-poc \
  --topic=glean-server-event-direct \
  --rate=1000
```

**High-throughput testing:**
```bash
go run main.go \
  --project=akomar-server-telemetry-poc \
  --topic=glean-server-event-direct \
  --rate=10000 \
  --duration=1m
```

**Command-line flags:**
- `--project` - GCP project ID (required)
- `--topic` - Pub/Sub topic name (required)
- `--rate` - Events per second to generate (default: 1)
- `--duration` - Run duration, e.g., `1m`, `30s` (default: infinite)

## [Experimental] Direct Publishing to Pub/Sub

### GKE Deployment

#### Prerequisites

**1. Create Pub/Sub topic:**
```bash
gcloud pubsub topics create glean-server-event-direct \
  --project=akomar-server-telemetry-poc
```

**2. Create test subscription (for monitoring):**
```bash
gcloud pubsub subscriptions create glean-direct-test \
  --topic=glean-server-event-direct \
  --project=akomar-server-telemetry-poc
```

**3. Setup Workload Identity for GKE authentication:**

```bash
# Create GCP service account
gcloud iam service-accounts create telemetry-publisher \
  --display-name="Telemetry Publisher" \
  --project=akomar-server-telemetry-poc

# Grant Pub/Sub publish permission
gcloud projects add-iam-policy-binding akomar-server-telemetry-poc \
  --member="serviceAccount:telemetry-publisher@akomar-server-telemetry-poc.iam.gserviceaccount.com" \
  --role="roles/pubsub.publisher"

# Enable workload identity on GKE cluster
gcloud container clusters update custom-fluentbit \
  --region=us-east1 \
  --workload-pool=akomar-server-telemetry-poc.svc.id.goog

# Update node pool
gcloud container node-pools update default-pool \
  --cluster=custom-fluentbit \
  --region=us-east1 \
  --workload-metadata=GKE_METADATA

# Link Kubernetes SA to GCP SA (Workload Identity)
gcloud iam service-accounts add-iam-policy-binding \
  telemetry-publisher@akomar-server-telemetry-poc.iam.gserviceaccount.com \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:akomar-server-telemetry-poc.svc.id.goog[default/telemetry-publisher]" \
  --project=akomar-server-telemetry-poc

# Create Kubernetes service account
kubectl create serviceaccount telemetry-publisher

# Annotate K8s SA with GCP SA
kubectl annotate serviceaccount telemetry-publisher \
  iam.gke.io/gcp-service-account=telemetry-publisher@akomar-server-telemetry-poc.iam.gserviceaccount.com
```

**4. Enable Workload Identity in deployment:**

Edit `../kubernetes/test-go-publisher-deploy.yaml` and uncomment these lines:
```yaml
metadata:
  annotations:
    iam.gke.io/gcp-service-account: telemetry-publisher@akomar-server-telemetry-poc.iam.gserviceaccount.com
spec:
  serviceAccountName: telemetry-publisher
```

#### Build and Deploy

**Build and push Docker image:**
```bash
# Set GCP configuration
export project_id=akomar-server-telemetry-poc


docker build --platform linux/amd64 -t test-go-publisher .
docker tag test-go-publisher gcr.io/${project_id}/test-go-publisher
docker push gcr.io/${project_id}/test-go-publisher
```

**Deploy to GKE:**
```bash
# Apply deployment
kubectl apply -f ../kubernetes/test-go-publisher-deploy.yaml

# Verify pod is running
kubectl get pods -l component=test-go-publisher

# Check pod status
kubectl describe pod -l component=test-go-publisher
```

#### Monitoring

**Watch application logs:**
```bash
# Follow logs (stats reported every 10 seconds)
kubectl logs -f deployment/test-go-publisher

# Expected output:
# Stats: Generated=100000 (10000/s), PublishCalls=100000, Blocked=0 (0.0%), Published=98000 (9800/s), Errors=0
```

**Verify messages in Pub/Sub:**
```bash
# Pull sample messages
gcloud pubsub subscriptions pull glean-direct-test \
  --project=akomar-server-telemetry-poc \
  --limit=5 \
  --auto-ack

# Check if decoder is processing successfully (if decoder job is running)
# Messages should appear in structured-decoded topic, not structured-errors
gcloud pubsub subscriptions pull structured-decoded-sub --limit=5 --auto-ack
```

**Check resource usage:**
```bash
# Monitor CPU and memory usage
kubectl top pod -l component=test-go-publisher
```

#### Stopping the Deployment

```bash
# Delete deployment (waits 30s for graceful shutdown + flush)
kubectl delete deployment test-go-publisher

# Or delete everything (deployment + configmap)
kubectl delete -f ../kubernetes/test-go-publisher-deploy.yaml
```

#### Scaling for Load Testing

**Test at different rates:**
```bash
# Lower rate: 1k msgs/s
kubectl patch configmap test-go-publisher-config -p '{"data":{"EVENTS_PER_SEC":"1000"}}'
kubectl rollout restart deployment/test-go-publisher

# Higher rate: 20k msgs/s (single pod)
kubectl patch configmap test-go-publisher-config -p '{"data":{"EVENTS_PER_SEC":"20000"}}'
kubectl rollout restart deployment/test-go-publisher
```

**Horizontal scaling for 40k msgs/s aggregate:**
```bash
# Set each pod to 1k msgs/s
kubectl patch configmap test-go-publisher-config -p '{"data":{"EVENTS_PER_SEC":"1000"}}'

# Scale to 40 replicas
kubectl scale deployment test-go-publisher --replicas=40

# Monitor aggregate throughput across all pods
kubectl logs -f -l component=test-go-publisher | grep "Stats:"
```

### Architecture

**Components:**
- `glean/server_events.go` - Self-contained Pub/Sub publisher (~400 lines)
  - All Glean types duplicated (no dependencies)
  - Asynchronous batched publishing
  - Flow control with backpressure
  - Cloud Logging envelope format for decoder compatibility

- `main.go` - Publisher application
  - Rate-based event generation (10ms ticks with catch-up loop)
  - Command-line flags and environment variable support
  - Stats reporting every 10 seconds
  - Graceful shutdown with message flushing

**Performance Configuration:**
- Batch size: Up to 1000 messages per publish
- Max batching delay: 100ms
- Flow control: 100k message buffer, 500MB memory limit
- Parallelism: 2×CPU goroutines for concurrent publishing

### Troubleshooting

**Pod fails to start:**
```bash
# Check pod events
kubectl describe pod -l component=test-go-publisher

# Common issues:
# - Image pull errors: Verify image exists in gcr.io
# - Workload Identity errors: Verify service account setup
# - Missing topic: Create glean-server-event-direct topic
```

**Low throughput:**
```bash
# Check CPU throttling
kubectl top pod -l component=test-go-publisher

# If CPU is maxed at 1000m, increase limit in deployment YAML:
# limits:
#   cpu: 2000m
```

**Messages going to error stream:**
```bash
# Verify message format
gcloud pubsub subscriptions pull glean-direct-test --limit=1

# Should have jsonPayload.Fields structure for ParseLogEntry compatibility
```

### Development Notes

**Regenerating Glean code:**

The `glean/server_events.go` file is based on code generated by glean_parser. In production, glean_parser would generate this file with Pub/Sub support. For now, it's manually maintained to match the stdout version from test-go-logger.

To keep in sync with test-go-logger changes:
```bash
# After regenerating test-go-logger/glean/server_events.go
# Manually sync shared types to test-go-publisher/glean/server_events.go:
# - RequestInfo, clientInfo, pingInfo, ping, metrics, pingPayload, gleanEvent
# - BackendObjectUpdateEvent, EventsPingEvent, EventsPing
# - createPingInfo(), newGleanEvent() helper functions
```
