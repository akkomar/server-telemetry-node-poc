package main

// Reference application for the direct Pub/Sub Glean telemetry path
// (DENG-9533). Demonstrates the intended split between glean_parser-generated
// code (glean/server_events.go: stateless message builder) and
// application-owned code (publisher.go: client / lifecycle / metrics, plus
// this file: wiring + a synthetic event loop for throughput testing).

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"test-go-publisher/glean"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// startMetricsServer is a test-harness convenience: it spins up a dedicated
// HTTP listener so we can curl /metrics during a local run. Real
// applications already serve /metrics from their main HTTP server (e.g.
// `mux.Handle("GET /metrics", promhttp.Handler())`); because publisher.go
// registers its metrics on the default Prometheus registry, that existing
// endpoint picks them up automatically with no extra wiring.
func startMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server: %v", err)
		}
	}()
	return srv
}

// TEST HARNESS counters. These measure the synthetic generator, not the
// publisher; real applications should rely on the Prometheus metrics
// registered in publisher.go.
var (
	generatedCount atomic.Int64
	publishErrors  atomic.Int64
)

func main() {
	projectID := flag.String("project", getEnv("PROJECT_ID", ""), "GCP project ID (required)")
	topicID := flag.String("topic", getEnv("TOPIC_ID", ""), "Pub/Sub topic name (required)")
	eventsPerSec := flag.Int("rate", getEnvInt("EVENTS_PER_SEC", 1), "Events per second to generate")
	duration := flag.Duration("duration", 0, "Run duration (0 = infinite)")
	metricsAddr := flag.String("metrics-addr", getEnv("METRICS_ADDR", ":2112"), "Prometheus /metrics listen address")
	flushTimeout := flag.Duration("flush-timeout", 30*time.Second, "Max wait for in-flight publishes on shutdown")
	flag.Parse()

	if *projectID == "" {
		log.Fatal("--project or PROJECT_ID is required")
	}
	if *topicID == "" {
		log.Fatal("--topic or TOPIC_ID is required")
	}

	log.Printf("starting direct Pub/Sub publisher: project=%s topic=%s rate=%d/s duration=%v",
		*projectID, *topicID, *eventsPerSec, *duration)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// === REFERENCE IMPL: startup ===
	// Build the Glean message builder and the publisher once at startup; reuse
	// for the process lifetime.
	builder := glean.GleanEventsBuilder{
		AppID:             "accounts_backend", // your Glean app ID
		AppDisplayVersion: "0.0.1",            // your build version
		AppChannel:        "dev",              // prod / stage / dev
	}

	publisher, err := NewPublisher(ctx, *projectID, *topicID)
	if err != nil {
		log.Fatalf("create publisher: %v", err)
	}

	metricsSrv := startMetricsServer(*metricsAddr)
	log.Printf("Prometheus /metrics on %s", *metricsAddr)

	// === REFERENCE IMPL: graceful shutdown ===
	// Drain in-flight publishes (bounded by --flush-timeout), then release
	// client + metrics server.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("received %s, draining...", sig)
		flushCtx, flushCancel := context.WithTimeout(context.Background(), *flushTimeout)
		defer flushCancel()
		if err := publisher.Flush(flushCtx); err != nil {
			log.Printf("flush: %v", err)
		}
		if err := publisher.Close(); err != nil {
			log.Printf("close: %v", err)
		}
		_ = metricsSrv.Shutdown(context.Background())
		os.Exit(0)
	}()

	// TEST HARNESS: 10-second heartbeat so kubectl logs can confirm the
	// generator is keeping up without scraping Prometheus.
	overallStart := time.Now()
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				gen := generatedCount.Load()
				errs := publishErrors.Load()
				elapsed := time.Since(overallStart).Seconds()
				log.Printf("generator: events=%d (%.0f/s), publish_errors=%d",
					gen, float64(gen)/elapsed, errs)
			case <-ctx.Done():
				return
			}
		}
	}()

	// TEST HARNESS: rate-paced event generator with catch-up. Replace with
	// your application's actual event-recording call sites.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	eventCount := 0
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(startTime).Seconds()
			expected := int(elapsed * float64(*eventsPerSec))
			for eventCount < expected {
				// === REFERENCE IMPL: per-event publish ===
				// Build a Glean ping message, hand it to the publisher. The
				// builder is stateless; multiple goroutines may share one.
				params := glean.EventsPing{
					IdentifiersFxaAccountId: fmt.Sprintf("account_%d", eventCount),
					Event: glean.BackendObjectUpdateEvent{
						ObjectType:  "api_request",
						ObjectState: fmt.Sprintf(`{"request_id": %d, "timestamp": %q}`, eventCount, time.Now().Format(time.RFC3339)),
						Linking:     eventCount%2 == 0,
					},
				}
				msg, err := builder.BuildEventsPingMessageWithoutUserInfo(params)
				if err != nil {
					publishErrors.Add(1)
					log.Printf("build message: %v", err)
					eventCount++
					continue
				}
				publisher.Publish(ctx, msg)
				generatedCount.Add(1)
				eventCount++
			}

			if *duration > 0 && time.Since(startTime) >= *duration {
				log.Printf("duration reached, draining...")
				flushCtx, flushCancel := context.WithTimeout(context.Background(), *flushTimeout)
				defer flushCancel()
				if err := publisher.Flush(flushCtx); err != nil {
					log.Printf("flush: %v", err)
				}
				if err := publisher.Close(); err != nil {
					log.Printf("close: %v", err)
				}
				_ = metricsSrv.Shutdown(context.Background())
				log.Printf("final: generated=%d errors=%d", generatedCount.Load(), publishErrors.Load())
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}
