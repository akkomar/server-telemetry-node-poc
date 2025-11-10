package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"test-go-http-publisher/glean"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Instrumentation counters to identify bottlenecks
var (
	generatedCount    atomic.Int64 // Total events generated
	publishCallCount  atomic.Int64 // Number of RecordEventsPing() calls made
	publishBlockCount atomic.Int64 // Number of times RecordEventsPing() blocked (>1ms)
)

// logPrometheusMetrics queries and logs Glean Prometheus metrics for testing/debugging
func logPrometheusMetrics() {
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		log.Printf("Error gathering metrics: %v", err)
		return
	}

	var queuedSuccess, queuedError int64
	var completedSuccess, completedError int64

	for _, mf := range metricFamilies {
		// Process both metrics to compare queued vs completed
		if mf.GetName() == "glean_http_publish_total" {
			for _, m := range mf.GetMetric() {
				labels := make(map[string]string)
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				val := int64(m.GetCounter().GetValue())
				if labels["status"] == "success" {
					queuedSuccess += val
				} else if labels["status"] == "error" {
					queuedError += val
				}
			}
		}

		if mf.GetName() == "glean_http_requests_completed" {
			for _, m := range mf.GetMetric() {
				labels := make(map[string]string)
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				val := int64(m.GetCounter().GetValue())
				if labels["status"] == "success" {
					completedSuccess += val
				} else if labels["status"] == "error" {
					completedError += val
				}
			}
		}
	}

	lag := queuedSuccess - completedSuccess
	log.Printf("Prometheus Metrics: Queued=%d, Completed=%d, Lag=%d, Errors=%d",
		queuedSuccess, completedSuccess, lag, queuedError+completedError)
}

func main() {
	// Command-line flags
	endpoint := flag.String("endpoint", getEnv("ENDPOINT", "https://stage.ingestion-edge.nonprod.dataops.mozgcp.net/submit"), "HTTP endpoint URL")
	eventsPerSec := flag.Int("rate", getEnvInt("EVENTS_PER_SEC", 1), "Events per second to generate")
	duration := flag.Duration("duration", 0, "Run duration (0 = infinite)")
	concurrency := flag.Int("concurrency", getEnvInt("CONCURRENCY", 10), "Number of concurrent HTTP workers")
	flag.Parse()

	if *endpoint == "" {
		log.Fatal("--endpoint or ENDPOINT environment variable is required")
	}

	log.Printf("Starting Glean HTTP publisher")
	log.Printf("Configuration: endpoint=%s, rate=%d events/sec, duration=%v, concurrency=%d",
		*endpoint, *eventsPerSec, *duration, *concurrency)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ============================================================================
	// PRODUCTION CODE: Initialize Glean HTTP Publisher
	// ============================================================================
	publisher, err := glean.NewGleanEventsPublisher(
		ctx,
		*endpoint,
		"accounts_backend", // Your application ID
		"0.0.1",            // Your application version
		"nightly",          // Your deployment channel (prod/stage/dev)
		*concurrency,       // Number of concurrent HTTP workers
	)
	if err != nil {
		log.Fatalf("Failed to create HTTP publisher: %v", err)
	}
	defer publisher.Close()
	// ============================================================================

	// TEST HARNESS: Track overall start time for rate calculation
	overallStartTime := time.Now()

	// Window tracking for rate calculation (last 30 seconds)
	type statsSnapshot struct {
		timestamp time.Time
		generated int64
		blocked   int64
	}
	var snapshots [3]statsSnapshot // 3 snapshots × 10s = 30s window
	snapshotIdx := 0

	// TEST HARNESS: Stats reporting (not needed in production - use Prometheus /metrics endpoint)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				generated := generatedCount.Load()
				publishCalls := publishCallCount.Load()
				blocked := publishBlockCount.Load()

				// Store current snapshot
				snapshots[snapshotIdx] = statsSnapshot{
					timestamp: now,
					generated: generated,
					blocked:   blocked,
				}

				// Calculate rate over last 30 seconds (using oldest snapshot)
				oldestIdx := (snapshotIdx + 1) % 3
				oldest := snapshots[oldestIdx]

				var actualRate float64
				var blockedPct float64
				if !oldest.timestamp.IsZero() && now.Sub(oldest.timestamp).Seconds() > 0 {
					window := now.Sub(oldest.timestamp).Seconds()
					generatedDelta := generated - oldest.generated
					blockedDelta := blocked - oldest.blocked
					actualRate = float64(generatedDelta) / window
					if generatedDelta > 0 {
						blockedPct = float64(blockedDelta) / float64(generatedDelta) * 100
					}
				} else {
					// First few iterations, calculate since start
					elapsed := now.Sub(overallStartTime).Seconds()
					actualRate = float64(generated) / elapsed
					if publishCalls > 0 {
						blockedPct = float64(blocked) / float64(publishCalls) * 100
					}
				}

				// Get queue depth to show backpressure
				queueDepth := publisher.QueueDepth()
				queuePct := float64(queueDepth) / 10000.0 * 100

				log.Printf("Stats: Generated=%d (%.0f/s), PublishCalls=%d, Blocked=%d (%.1f%%), Queue=%d/10000 (%.1f%%)",
					generated, actualRate, publishCalls, blocked, blockedPct, queueDepth, queuePct)
				logPrometheusMetrics()

				// Move to next snapshot position
				snapshotIdx = (snapshotIdx + 1) % 3
			case <-ctx.Done():
				return
			}
		}
	}()

	// ============================================================================
	// PRODUCTION CODE: Graceful Shutdown Handler
	// ============================================================================
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, flushing pending messages...")
		publisher.Flush()
		os.Exit(0)
	}()
	// ============================================================================

	// TEST HARNESS: Event generation loop (replace with your application logic)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	eventCount := 0

	for {
		select {
		case <-ticker.C:
			// Calculate how many events we SHOULD have generated by now based on elapsed time
			elapsed := time.Since(startTime).Seconds()
			expectedCount := int(elapsed * float64(*eventsPerSec))

			// Generate all events we're "behind" on (burst generation)
			for eventCount < expectedCount {
				// TEST HARNESS: Track event generation
				generatedCount.Add(1)
				publishStart := time.Now()

				// ================================================================
				// PRODUCTION CODE: Record Glean Event
				// ================================================================
				requestInfo := glean.RequestInfo{}
				params := glean.EventsPing{
					IdentifiersFxaAccountId: fmt.Sprintf("account_%d", eventCount),
					Event: glean.BackendObjectUpdateEvent{
						ObjectType:  "api_request",
						ObjectState: fmt.Sprintf(`{"request_id": %d, "timestamp": "%s"}`, eventCount, time.Now().Format(time.RFC3339)),
						Linking:     eventCount%2 == 0,
					},
				}

				if err := publisher.RecordEventsPing(requestInfo, params); err != nil {
					log.Printf("Error recording event: %v", err)
				}
				// ================================================================

				// TEST HARNESS: Track instrumentation
				publishCallCount.Add(1)
				publishDuration := time.Since(publishStart)
				if publishDuration > 1*time.Millisecond {
					publishBlockCount.Add(1)
				}

				eventCount++
			}

			if *duration > 0 && time.Since(startTime) >= *duration {
				elapsed := time.Since(startTime).Seconds()
				generated := generatedCount.Load()
				publishCalls := publishCallCount.Load()
				blocked := publishBlockCount.Load()

				blockedPct := 0.0
				if publishCalls > 0 {
					blockedPct = float64(blocked) / float64(publishCalls) * 100
				}

				queueDepth := publisher.QueueDepth()
				queuePct := float64(queueDepth) / 10000.0 * 100

				log.Printf("=== FINAL STATS ===")
				log.Printf("Duration: %.2fs", elapsed)
				log.Printf("Target rate: %d msgs/s", *eventsPerSec)
				log.Printf("Generated: %d events (expected: %d, actual rate: %.0f/s)",
					generated, int(elapsed*float64(*eventsPerSec)), float64(generated)/elapsed)
				log.Printf("PublishCalls: %d", publishCalls)
				log.Printf("Blocked: %d times (%.1f%% of calls took >1ms)", blocked, blockedPct)
				log.Printf("Queue: %d/10000 (%.1f%% full)", queueDepth, queuePct)
				log.Printf("Concurrency: %d workers", *concurrency)
				logPrometheusMetrics()
				log.Printf("==================")
				log.Printf("Note: Production applications should expose metrics via HTTP:")
				log.Printf("  http.Handle(\"/metrics\", promhttp.Handler())")

				publisher.Flush()
				return
			}

		case <-ctx.Done():
			log.Printf("Context cancelled, generated %d events", eventCount)
			return
		}
	}
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns environment variable as int or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
