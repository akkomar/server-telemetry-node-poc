package main

// Publisher is the reference implementation for the transport / lifecycle /
// observability half of the direct Pub/Sub Glean telemetry path (DENG-9533).
// glean_parser generates only the Glean-specific message construction in
// glean/server_events.go (a stateless GleanEventsBuilder); this file shows
// the patterns an application is expected to layer on top:
//
//   - a single shared pubsub.Client + pubsub.Publisher, with batching configured
//   - a synchronous (blocking) Publish that waits for the Pub/Sub ack and
//     returns the result, so the caller can fall back or surface the error
//     inline. This mirrors MARS's server publisher in
//     internal/gcp/pubsub/provider.go.
//   - Close to flush the batcher and release the gRPC connection
//   - Prometheus metrics covering both publisher-side flow control and
//     end-to-end Pub/Sub round-trip
//
// treat the comments here as the rationale for the patterns, not boilerplate.

import (
	"context"
	"fmt"
	"time"

	pubsub "cloud.google.com/go/pubsub/v2"
	vkit "cloud.google.com/go/pubsub/v2/apiv1"
	"github.com/googleapis/gax-go/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/codes"
)

// Publisher owns a Pub/Sub client + publisher and provides a synchronous
// Publish plus shutdown sequencing and Prometheus instrumentation.
//
// Caller contract: ensure all Publish calls have returned before invoking
// Close. Concurrent Publish + Close races a panic in the Pub/Sub client.
// The recommended shutdown sequence is "stop accepting new work → wait for
// in-flight Publish calls to return → Close" (see the Close doc comment).
type Publisher struct {
	client *pubsub.Client
	topic  *pubsub.Publisher

	// publishTimeout bounds how long a single Publish call blocks waiting for
	// the Pub/Sub ack before failing, so a stuck publish can't hang the caller.
	publishTimeout time.Duration
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
		Help:    "Latency of the topic.Publish enqueue (handing the message to the batcher), before the blocking wait for the ack.",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 14), // 0.1ms .. 1.6s
	})

	publishResultDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "glean_pubsub_result_duration_seconds",
		Help:    "Total time a Publish call blocks: enqueue + batching delay + round-trip to the Pub/Sub ack/error. This is the latency the caller experiences.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 14), // 1ms .. 16s
	})

	publishInflight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "glean_pubsub_inflight",
		Help: "Publish calls currently blocked awaiting their Pub/Sub result.",
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

// defaultPublishTimeout bounds how long Publish blocks waiting for an ack. On a
// latency-sensitive hot path, lower this (a few hundred ms) so slow publishes
// fail fast into your fallback instead of inflating request latency - this is
// the synchronous-publish tradeoff discussed in the MARS cutover plan.
const defaultPublishTimeout = 30 * time.Second

// NewPublisher constructs a Publisher. The caller owns shutdown: stop
// publishing, let in-flight Publish calls return, then call Close.
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
	// the caller forever. Library default is 60s; 30s is the same bound MARS
	// uses for its existing publishers.
	topic.PublishSettings.Timeout = 30 * time.Second

	return &Publisher{client: client, topic: topic, publishTimeout: defaultPublishTimeout}, nil
}

// Publish sends msg and BLOCKS until Pub/Sub acks it, or the publish errors or
// exceeds publishTimeout, returning the outcome. This mirrors MARS's server
// publisher (internal/gcp/pubsub/provider.go): the publish is synchronous so
// the caller learns the result inline and can fall back (e.g. to stdout
// logging) or surface the error.
//
// Call it from your request-handler goroutine. Do NOT funnel every publish
// through a single goroutine: a blocking publish only sustains throughput when
// many callers run it concurrently, which is exactly what a server gets from
// concurrent request handlers. The call is bounded by publishTimeout (applied
// on top of the caller's ctx), so a stuck or slow publish fails fast rather
// than hanging the caller.
func (p *Publisher) Publish(ctx context.Context, msg *pubsub.Message) error {
	start := time.Now()
	publishInflight.Inc()
	defer publishInflight.Dec()

	publishCtx, cancel := context.WithTimeout(ctx, p.publishTimeout)
	defer cancel()

	result := p.topic.Publish(publishCtx, msg)
	publishCallDuration.Observe(time.Since(start).Seconds())

	_, err := result.Get(publishCtx)
	publishResultDuration.Observe(time.Since(start).Seconds())
	if err != nil {
		publishTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("pubsub publish: %w", err)
	}
	publishTotal.WithLabelValues("success").Inc()
	return nil
}

// Close stops the publisher's batcher (blocks until any pending batches
// flush) and releases the gRPC client. Must not race with Publish: stop
// accepting new publishes and let in-flight Publish calls return first.
//
// Recommended shutdown sequence:
//
//	// stop accepting new work and wait for in-flight Publish calls to return,
//	// then:
//	_ = publisher.Close()
func (p *Publisher) Close() error {
	p.topic.Stop()
	return p.client.Close()
}
