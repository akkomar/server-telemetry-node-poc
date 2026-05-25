package main

// Publisher is the reference implementation for the transport / lifecycle /
// observability half of the direct Pub/Sub Glean telemetry path (DENG-9533).
// glean_parser generates only the Glean-specific message construction in
// glean/server_events.go (a stateless GleanEventsBuilder); this file shows
// the patterns an application is expected to layer on top:
//
//   - a single shared pubsub.Client + pubsub.Publisher, with batching configured
//   - async result handling via goroutines (success/error metrics, not
//     per-message handshake)
//   - context-bounded Flush so callers can drain on shutdown without hanging
//     forever, and Close to release the gRPC connection
//   - Prometheus metrics covering both publisher-side flow control and
//     end-to-end Pub/Sub round-trip
//
// treat the comments here as the rationale for the patterns, not boilerplate.

import (
	"context"
	"fmt"
	"sync"
	"time"

	pubsub "cloud.google.com/go/pubsub/v2"
	vkit "cloud.google.com/go/pubsub/v2/apiv1"
	"github.com/googleapis/gax-go/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/codes"
)

// Publisher owns a Pub/Sub client + publisher and handles async result
// tracking, shutdown sequencing, and Prometheus instrumentation.
//
// Caller contract: ensure all Publish calls have completed before invoking
// Close. Concurrent Publish + Close races a panic in the Pub/Sub client.
// The recommended shutdown sequence is "stop accepting new work → Flush →
// Close" (see the Close doc comment).
type Publisher struct {
	client *pubsub.Client
	topic  *pubsub.Publisher

	// wg tracks in-flight result goroutines so Flush can wait on them.
	wg sync.WaitGroup
}

// Prometheus metrics. Registered against the default registry via promauto
// so that an existing promhttp.Handler() on /metrics (as MARS already
// serves) exposes them automatically with no extra wiring.
var (
	publishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "glean_pubsub_publish_total",
		Help: "Total Pub/Sub publish results, labelled by terminal status.",
	}, []string{"status"})

	publishCallDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "glean_pubsub_publish_duration_seconds",
		Help:    "Latency of the synchronous publisher.Publish call. Spikes indicate publisher-side flow control / backpressure.",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 14), // 0.1ms .. 1.6s
	})

	publishResultDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "glean_pubsub_result_duration_seconds",
		Help:    "End-to-end latency from publish call to Pub/Sub ack/error. Includes batching delay and network round-trip.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 14), // 1ms .. 16s
	})

	publishInflight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "glean_pubsub_inflight",
		Help: "Pub/Sub publishes currently awaiting result.",
	})
)

// retryableCodes is the gRPC error set considered safe to retry. Matches the
// configuration MARS uses for its existing publishers
// (internal/gcp/pubsub/provider.go); informed by their production experience.
var retryableCodes = []codes.Code{
	codes.Aborted,
	codes.Canceled,
	codes.Internal,
	codes.ResourceExhausted,
	codes.Unknown,
	codes.Unavailable,
	codes.DeadlineExceeded,
}

// NewPublisher constructs a Publisher. The caller owns shutdown: call Flush
// to wait for in-flight messages, then Close to release the gRPC client.
//
// Batching / flow control defaults target ~10k msgs/s per process. Adjust
// for your workload; see
// https://pkg.go.dev/cloud.google.com/go/pubsub/v2#PublishSettings.
//
// Bounded buffering is intentionally not configured (the library default is
// effectively unbounded). Whether "drop during sustained Pub/Sub outage" is
// acceptable depends on the data: it might be fine for diagnostic telemetry
// and unacceptable for anything with billing or accounting semantics. If you
// need explicit backpressure, set pubsub.FlowControlSettings on the topic's
// PublishSettings (MaxOutstandingMessages / MaxOutstandingBytes with
// LimitExceededBehavior: pubsub.FlowControlBlock).
func NewPublisher(ctx context.Context, projectID, topicID string) (*Publisher, error) {
	client, err := pubsub.NewClientWithConfig(ctx, projectID, &pubsub.ClientConfig{
		TopicAdminCallOptions: &vkit.TopicAdminCallOptions{
			Publish: []gax.CallOption{
				gax.WithRetry(func() gax.Retryer {
					return gax.OnCodes(retryableCodes, gax.Backoff{
						Initial:    333 * time.Millisecond,
						Max:        1 * time.Second,
						Multiplier: 2.0,
					})
				}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("pubsub.NewClientWithConfig: %w", err)
	}
	topic := client.Publisher(topicID)

	topic.PublishSettings.CountThreshold = 1000
	topic.PublishSettings.DelayThreshold = 100 * time.Millisecond
	// Bound total time the client spends on a single message (including
	// retries) so a stuck publish surfaces as an error instead of blocking
	// the result-await goroutine forever. Library default is 60s; 30s is
	// the same bound MARS uses for its existing publishers.
	topic.PublishSettings.Timeout = 30 * time.Second

	return &Publisher{client: client, topic: topic}, nil
}

// Publish hands msg to the Pub/Sub client's batcher and returns once the
// batcher has accepted it. The synchronous path doesn't surface errors;
// success / error is recorded asynchronously via Prometheus.
func (p *Publisher) Publish(ctx context.Context, msg *pubsub.Message) {
	publishStart := time.Now()
	result := p.topic.Publish(ctx, msg)
	publishCallDuration.Observe(time.Since(publishStart).Seconds())

	publishInflight.Inc()
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer publishInflight.Dec()
		_, err := result.Get(context.Background())
		publishResultDuration.Observe(time.Since(publishStart).Seconds())
		if err != nil {
			publishTotal.WithLabelValues("error").Inc()
		} else {
			publishTotal.WithLabelValues("success").Inc()
		}
	}()
}

// Flush waits for all in-flight publish results to land (success or error).
// Returns nil on clean drain, ctx.Err() if the context expires first. Safe
// to call multiple times.
func (p *Publisher) Flush(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close stops the publisher's batcher (blocks until any pending batches
// flush) and releases the gRPC client. Must not race with Publish: stop
// accepting new publishes first.
//
// Recommended shutdown sequence:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//	_ = publisher.Flush(ctx)
//	_ = publisher.Close()
func (p *Publisher) Close() error {
	p.topic.Stop()
	return p.client.Close()
}
