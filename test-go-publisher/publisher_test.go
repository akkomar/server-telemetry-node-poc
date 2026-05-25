package main

// Smoke tests using the in-process Pub/Sub fake (pstest). Validates that
// what the publisher actually puts on the wire matches the contract the
// gcp-ingestion direct-Pub/Sub decoder expects: required document_*
// attributes, gzipped JSON body, optional user_agent / x_forwarded_for.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	pubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"cloud.google.com/go/pubsub/v2/pstest"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"test-go-publisher/glean"
)

// setupFake starts a pstest server, creates the topic, and returns a
// Publisher pointed at it. Caller is responsible for srv.Close().
func setupFake(t *testing.T, topicID string) (*pstest.Server, *Publisher) {
	t.Helper()
	srv := pstest.NewServer()
	t.Cleanup(func() { srv.Close() })
	t.Setenv("PUBSUB_EMULATOR_HOST", srv.Addr)

	ctx := context.Background()
	conn, err := grpc.Dial(srv.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial pstest: %v", err)
	}
	setupClient, err := pubsub.NewClient(ctx, "test-project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("setup client: %v", err)
	}
	if _, err := setupClient.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: "projects/test-project/topics/" + topicID,
	}); err != nil {
		t.Fatalf("create topic: %v", err)
	}
	_ = setupClient.Close()
	_ = conn.Close()

	pub, err := NewPublisher(ctx, "test-project", topicID)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	return srv, pub
}

// drain runs the recommended Flush -> Close sequence with a bounded timeout.
func drain(t *testing.T, pub *Publisher) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pub.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// newBuilder returns the builder configuration used across tests.
func newBuilder() glean.GleanEventsBuilder {
	return glean.GleanEventsBuilder{
		AppID:             "accounts_backend",
		AppDisplayVersion: "0.0.1",
		AppChannel:        "nightly",
	}
}

// sampleEvent constructs a minimal valid EventsPing payload.
func sampleEvent(accountID string) glean.EventsPing {
	return glean.EventsPing{
		IdentifiersFxaAccountId: accountID,
		Event: glean.BackendObjectUpdateEvent{
			ObjectType:  "api_request",
			ObjectState: `{"request_id":1}`,
			Linking:     true,
		},
	}
}

func TestBuildAndPublish_Attributes(t *testing.T) {
	srv, pub := setupFake(t, "structured-direct")
	builder := newBuilder()

	msg, err := builder.BuildEventsPingMessageWithoutUserInfo(sampleEvent("acct_1"))
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	pub.Publish(context.Background(), msg)
	drain(t, pub)

	msgs := srv.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got := msgs[0]

	// Required attributes per decoder contract.
	if got.Attributes["document_namespace"] != "accounts_backend" {
		t.Errorf("document_namespace=%q, want %q", got.Attributes["document_namespace"], "accounts_backend")
	}
	if got.Attributes["document_type"] != "events" {
		t.Errorf("document_type=%q, want %q", got.Attributes["document_type"], "events")
	}
	if got.Attributes["document_version"] != "1" {
		t.Errorf("document_version=%q, want %q", got.Attributes["document_version"], "1")
	}
	if _, err := uuid.Parse(got.Attributes["document_id"]); err != nil {
		t.Errorf("document_id %q not a valid UUID: %v", got.Attributes["document_id"], err)
	}

	// Optional attributes should be absent when RequestInfo is empty.
	if v, ok := got.Attributes["user_agent"]; ok {
		t.Errorf("user_agent should be absent, got %q", v)
	}
	if v, ok := got.Attributes["x_forwarded_for"]; ok {
		t.Errorf("x_forwarded_for should be absent, got %q", v)
	}

	// Publisher does NOT stamp submission_timestamp; decoder uses publishTime.
	if v, ok := got.Attributes["submission_timestamp"]; ok {
		t.Errorf("submission_timestamp should be absent on the wire, got %q", v)
	}
}

func TestBuildAndPublish_PayloadIsGzippedJSON(t *testing.T) {
	srv, pub := setupFake(t, "structured-direct")
	builder := newBuilder()

	msg, err := builder.BuildEventsPingMessageWithoutUserInfo(sampleEvent("acct_42"))
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	pub.Publish(context.Background(), msg)
	drain(t, pub)

	msgs := srv.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	gz, err := gzip.NewReader(bytes.NewReader(msgs[0].Data))
	if err != nil {
		t.Fatalf("data is not valid gzip: %v", err)
	}
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("inner JSON parse: %v", err)
	}

	for _, key := range []string{"client_info", "ping_info", "metrics", "events"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("missing pingPayload field %q", key)
		}
	}

	clientInfo, _ := payload["client_info"].(map[string]any)
	if clientInfo["app_display_version"] != "0.0.1" {
		t.Errorf("client_info.app_display_version=%v, want 0.0.1", clientInfo["app_display_version"])
	}
	if clientInfo["app_channel"] != "nightly" {
		t.Errorf("client_info.app_channel=%v, want nightly", clientInfo["app_channel"])
	}

	events, _ := payload["events"].([]any)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0].(map[string]any)
	if ev["category"] != "backend" || ev["name"] != "object_update" {
		t.Errorf("event identity wrong: category=%v name=%v", ev["category"], ev["name"])
	}
}

func TestBuildAndPublish_RequestInfoAttributes(t *testing.T) {
	srv, pub := setupFake(t, "structured-direct")
	builder := newBuilder()

	msg, err := builder.BuildEventsPingMessage(
		glean.RequestInfo{
			UserAgent: "Mozilla/5.0 (test)",
			IpAddress: "203.0.113.1",
		},
		sampleEvent("acct_7"),
	)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	pub.Publish(context.Background(), msg)
	drain(t, pub)

	got := srv.Messages()[0]
	if got.Attributes["user_agent"] != "Mozilla/5.0 (test)" {
		t.Errorf("user_agent=%q", got.Attributes["user_agent"])
	}
	if got.Attributes["x_forwarded_for"] != "203.0.113.1" {
		t.Errorf("x_forwarded_for=%q", got.Attributes["x_forwarded_for"])
	}
}

func TestPublisher_FlushDrainsInFlight(t *testing.T) {
	srv, pub := setupFake(t, "structured-direct")
	builder := newBuilder()

	const n = 20
	for i := 0; i < n; i++ {
		msg, err := builder.BuildEventsPingMessageWithoutUserInfo(sampleEvent("acct"))
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		pub.Publish(context.Background(), msg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pub.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if got := len(srv.Messages()); got != n {
		t.Errorf("expected %d messages after Flush, got %d", n, got)
	}
	if got := testutil.ToFloat64(publishInflight); got != 0 {
		t.Errorf("publishInflight after drain = %v, want 0", got)
	}
}

// Regression test: cancelling the caller's ctx after Publish must not
// produce a spurious "error" metric. The underlying RPC runs on a
// detached context (see publisher.Publish doc comment); ctx cancellation
// after enqueue must not flip a successful publish into a metric error.
func TestPublisher_CallerCtxCancellationDoesNotRecordError(t *testing.T) {
	_, pub := setupFake(t, "structured-direct")
	builder := newBuilder()

	errBefore := testutil.ToFloat64(publishTotal.WithLabelValues("error"))
	succBefore := testutil.ToFloat64(publishTotal.WithLabelValues("success"))

	msg, err := builder.BuildEventsPingMessageWithoutUserInfo(sampleEvent("acct"))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	pub.Publish(ctx, msg)
	cancel() // immediately after enqueue; batcher has not yet flushed
	drain(t, pub)

	if got := testutil.ToFloat64(publishTotal.WithLabelValues("error")) - errBefore; got != 0 {
		t.Errorf("error counter delta = %v, want 0 (caller ctx cancellation should not record an error)", got)
	}
	if got := testutil.ToFloat64(publishTotal.WithLabelValues("success")) - succBefore; got != 1 {
		t.Errorf("success counter delta = %v, want 1", got)
	}
}

// A publish that fails on the wire must increment publish_total{status="error"}.
// Injects a non-retryable gRPC error (InvalidArgument is not in
// retryableCodes) so the publish fails on the first attempt and bubbles
// straight to result.Get.
func TestPublisher_PublishFailureRecordsError(t *testing.T) {
	srv, pub := setupFake(t, "structured-direct")
	srv.SetAutoPublishResponse(false)
	srv.AddPublishResponse(nil, status.Error(codes.InvalidArgument, "simulated"))

	builder := newBuilder()
	msg, err := builder.BuildEventsPingMessageWithoutUserInfo(sampleEvent("acct"))
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	errBefore := testutil.ToFloat64(publishTotal.WithLabelValues("error"))
	pub.Publish(context.Background(), msg)
	drain(t, pub)

	if got := testutil.ToFloat64(publishTotal.WithLabelValues("error")) - errBefore; got != 1 {
		t.Errorf("error counter delta = %v, want 1", got)
	}
}

