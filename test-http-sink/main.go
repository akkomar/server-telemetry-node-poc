package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_sink_requests_total",
			Help: "Total number of HTTP requests received",
		},
		[]string{"status"},
	)

	requestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_sink_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
)

// Stats counters
var (
	totalRequests atomic.Int64
	totalBytes    atomic.Int64
	startTime     time.Time
)

// Window tracking for rate calculation (last 30 seconds)
type windowSnapshot struct {
	timestamp time.Time
	requests  int64
	bytes     int64
}

func main() {
	startTime = time.Now()

	// Keep snapshots for 30-second window rate calculation
	var snapshots [3]windowSnapshot // 3 snapshots × 10s = 30s window
	snapshotIdx := 0

	// Stats reporting goroutine
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			currentRequests := totalRequests.Load()
			currentBytes := totalBytes.Load()

			// Store current snapshot
			snapshots[snapshotIdx] = windowSnapshot{
				timestamp: now,
				requests:  currentRequests,
				bytes:     currentBytes,
			}

			// Calculate rate over last 30 seconds (using oldest snapshot)
			oldestIdx := (snapshotIdx + 1) % 3
			oldest := snapshots[oldestIdx]

			var rps, mbps float64
			if !oldest.timestamp.IsZero() && now.Sub(oldest.timestamp).Seconds() > 0 {
				window := now.Sub(oldest.timestamp).Seconds()
				requestsDelta := currentRequests - oldest.requests
				bytesDelta := currentBytes - oldest.bytes
				rps = float64(requestsDelta) / window
				mbps = float64(bytesDelta) / window / 1024 / 1024
			} else {
				// First few iterations, calculate since start
				elapsed := now.Sub(startTime).Seconds()
				rps = float64(currentRequests) / elapsed
				mbps = float64(currentBytes) / elapsed / 1024 / 1024
			}

			log.Printf("Stats: Requests=%d (%.0f req/s), Data=%.2f MB (%.2f MB/s)",
				currentRequests, rps, float64(currentBytes)/1024/1024, mbps)

			// Move to next snapshot position
			snapshotIdx = (snapshotIdx + 1) % 3
		}
	}()

	// Health check endpoints
	http.HandleFunc("/__heartbeat__", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/__lbheartbeat__", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Main ingestion endpoint (matches ingestion-edge format)
	// POST /submit/{namespace}/{docType}/{version}/{id}
	http.HandleFunc("/submit/", handleSubmit)

	// Catch-all for other paths
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not Found")
	})

	log.Println("Starting HTTP sink server on :8080")
	log.Println("Ready to receive telemetry at /submit/{namespace}/{docType}/{version}/{id}")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	// Track in-flight requests
	requestsInFlight.Inc()
	defer requestsInFlight.Dec()

	// Only accept POST
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		requestsTotal.WithLabelValues("405").Inc()
		return
	}

	// Read and discard body (simulate processing but don't actually do anything)
	bodyBytes, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		requestsTotal.WithLabelValues("400").Inc()
		return
	}

	// Update stats
	totalRequests.Add(1)
	totalBytes.Add(int64(len(bodyBytes)))

	// Success response
	w.WriteHeader(http.StatusOK)
	requestsTotal.WithLabelValues("200").Inc()
}
