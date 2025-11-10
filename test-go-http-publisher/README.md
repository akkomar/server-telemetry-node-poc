# test-go-http-publisher

HTTP-based Glean telemetry publisher for testing throughput to Mozilla's ingestion endpoint.

## Purpose

This tool tests the feasibility of using HTTP ingestion as an alternative to direct Pub/Sub publishing for server-side Glean events, particularly for cross-region deployments where Pub/Sub inter-region fees are expensive.

Motivated by: SVCSE-3450 - Inter-region data delivery costs from EMEA to North America

## Key Differences from test-go-publisher

| Aspect | test-go-publisher (Pub/Sub) | test-go-http-publisher (HTTP) |
|--------|----------------------------|-------------------------------|
| Transport | Direct Pub/Sub | HTTP POST to ingestion-edge |
| Batching | Automatic (1000 msgs/batch) | Per-request (no batching) |
| Concurrency | Async with result channel | Async with worker pool |

## Usage

### Build

```bash
go build -o test-go-http-publisher .
```

### Run with Default Settings

```bash
./test-go-http-publisher \
  --endpoint=https://stage.ingestion-edge.nonprod.dataops.mozgcp.net/submit \
  --rate=100 \
  --duration=1m \
  --concurrency=10
```

### Command-Line Flags

- `--endpoint` - HTTP endpoint URL (default: https://stage.ingestion-edge.nonprod.dataops.mozgcp.net/submit)
- `--rate` - Events per second to generate (default: 1)
- `--duration` - Run duration, e.g., `30s`, `5m` (default: infinite)
- `--concurrency` - Number of concurrent HTTP workers (default: 10)

### Environment Variables

Alternatively, use environment variables:
- `ENDPOINT` - HTTP endpoint URL
- `EVENTS_PER_SEC` - Events per second
- `CONCURRENCY` - Number of workers

## Architecture

### HTTP Transport (glean/server_events.go)

- **Worker Pool:** N goroutines process HTTP requests concurrently
- **Async Pattern:** RecordEventsPing() returns immediately, queues request
- **Connection Pooling:** HTTP client maintains idle connections
- **Prometheus Metrics:** Tracks success/error counts

### Main Test Harness (main.go)

- **Rate Control:** Generates events at specified rate
- **Stats Reporting:** Logs throughput every 10 seconds
- **Graceful Shutdown:** Flushes pending requests on SIGTERM/SIGINT

## Test results

On a 2 vcpu pod with 1KB payloads and 10 goroutines, this achieves ~8k req/s sustained throughput to a mock HTTP endpoint over HTTP/1.1. Production ingestion-edge supports HTTP/2 which may improve performance.
