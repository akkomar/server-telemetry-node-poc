// Run with: go test -v -bench=. -benchmem
//
// Reports two things:
//   1) TestEnvelopeSizes: sizes of representative ads_backend Glean pings in three forms —
//      raw envelope, Cloud Logging-wrapped (current production path), and gzipped raw envelope.
//   2) BenchmarkGzip: gzip throughput at default and BestSpeed levels.
//
// The envelopes are constructed to land in the size ranges measured on the production
// `structured-logging` Pub/Sub topic for MARS ads_backend in Feb 2026:
//   interaction_v1   ~1.2 KB body (565 GB / 467M valid_submission)
//   request_stats_v1 ~0.78 KB body (1.945 TB / 2.5B valid_submission)
package benchmarks

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func mockUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// --- Glean envelope structures (mirror what glean_parser go_server template emits) ---

type clientInfo struct {
	TelemetrySDKBuild string `json:"telemetry_sdk_build"`
	FirstRunDate      string `json:"first_run_date"`
	OS                string `json:"os"`
	OSVersion         string `json:"os_version"`
	Architecture      string `json:"architecture"`
	AppBuild          string `json:"app_build"`
	AppDisplayVersion string `json:"app_display_version"`
	AppChannel        string `json:"app_channel"`
}

type pingInfo struct {
	Seq       int    `json:"seq"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type gleanEvent struct {
	Category  string            `json:"category"`
	Name      string            `json:"name"`
	Timestamp int64             `json:"timestamp"`
	Extra     map[string]string `json:"extra"`
}

type pingPayload struct {
	ClientInfo clientInfo                        `json:"client_info"`
	PingInfo   pingInfo                          `json:"ping_info"`
	Metrics    map[string]map[string]interface{} `json:"metrics"`
	Events     []gleanEvent                      `json:"events"`
}

type envelope struct {
	DocumentNamespace string `json:"document_namespace"`
	DocumentType      string `json:"document_type"`
	DocumentVersion   string `json:"document_version"`
	DocumentID        string `json:"document_id"`
	UserAgent         string `json:"user_agent,omitempty"`
	IPAddress         string `json:"ip_address,omitempty"`
	Payload           string `json:"payload"` // payload is a JSON-encoded string
}

func newClientInfo() clientInfo {
	return clientInfo{
		TelemetrySDKBuild: "glean_parser v18.1.0",
		FirstRunDate:      "2026-02-26T00:00:00.000Z",
		OS:                "Linux",
		OSVersion:         "5.15.0",
		Architecture:      "x86_64",
		AppBuild:          "1",
		AppDisplayVersion: "0.0.1",
		AppChannel:        "release",
	}
}

func newPingInfo() pingInfo {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	return pingInfo{Seq: 0, StartTime: now, EndTime: now}
}

func buildEnvelope(docType string, payload pingPayload) envelope {
	payloadJSON, _ := json.Marshal(payload)
	return envelope{
		DocumentNamespace: "ads_backend",
		DocumentType:      docType,
		DocumentVersion:   "1",
		DocumentID:        mockUUID(),
		Payload:           string(payloadJSON),
	}
}

// request_stats_v1: counters + summary distributions of request stats per pod.
// Real avg ~780B body, so payload metrics should be modestly sized.
func buildRequestStatsV1() envelope {
	return buildEnvelope("request_stats_v1", pingPayload{
		ClientInfo: newClientInfo(),
		PingInfo:   newPingInfo(),
		Metrics: map[string]map[string]interface{}{
			"counter": {
				"request_stats.requests_total":     12453,
				"request_stats.responses_2xx":      11872,
				"request_stats.responses_4xx":      431,
				"request_stats.responses_5xx":      150,
				"request_stats.timeouts":           27,
				"request_stats.upstream_errors":    19,
			},
			"timing_distribution": {
				"request_stats.latency_ms": map[string]interface{}{
					"sum":    1287634,
					"count":  12453,
					"values": map[string]int{"5": 200, "10": 1400, "25": 5300, "50": 3800, "100": 1200, "250": 380, "500": 120, "1000": 35, "2500": 12, "5000": 5, "10000": 3},
				},
			},
		},
		Events: nil,
	})
}

// interaction_v1: typically carries per-request interaction events.
// Real avg ~1.2 KB body, includes several events with extras.
func buildInteractionV1() envelope {
	makeEvent := func(name string, extra map[string]string) gleanEvent {
		return gleanEvent{Category: "interaction", Name: name, Timestamp: time.Now().UnixMilli(), Extra: extra}
	}
	return buildEnvelope("interaction_v1", pingPayload{
		ClientInfo: newClientInfo(),
		PingInfo:   newPingInfo(),
		Metrics: map[string]map[string]interface{}{
			"string": {
				"interaction.request_id":     mockUUID(),
				"interaction.advertiser_id":  "amzn-1234567890",
				"interaction.placement_id":   "newtab_sponsored_tile_1",
				"interaction.country_code":   "US",
			},
			"counter": {
				"interaction.impressions": 1,
			},
		},
		Events: []gleanEvent{
			makeEvent("impression", map[string]string{
				"position":    "tile_1",
				"surface":     "newtab",
				"ad_format":   "sponsored_tile",
				"placement":   "row_1_col_3",
				"viewability": "above_fold",
			}),
			makeEvent("click", map[string]string{
				"position":     "tile_1",
				"target_url":   "https://example.com/promotion/spring-sale-2026",
				"client_dwell": "2.4",
			}),
		},
	})
}

// --- Cloud Logging LogEntry wrapper (what the log sink ships to Pub/Sub) ---
//
// Source: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
// Resource labels for a GKE container come from the cluster autodetection.
// `jsonPayload` carries the original stdout-line JSON (i.e. the Glean envelope, parsed).

type logEntryResource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

type logEntryHTTP struct {
	// http_request not present for stdout logs; included only if your app sets it
}

type logEntry struct {
	LogName          string                 `json:"logName"`
	Resource         logEntryResource       `json:"resource"`
	Timestamp        string                 `json:"timestamp"`
	ReceiveTimestamp string                 `json:"receiveTimestamp"`
	Severity         string                 `json:"severity"`
	InsertID         string                 `json:"insertId"`
	Labels           map[string]string      `json:"labels,omitempty"`
	JsonPayload      map[string]interface{} `json:"jsonPayload"`
}

func wrapInLogEntry(env envelope) logEntry {
	// The log sink parses the stdout JSON line (the Glean envelope) into jsonPayload.
	var jp map[string]interface{}
	envJSON, _ := json.Marshal(env)
	_ = json.Unmarshal(envJSON, &jp)

	now := time.Now().UTC().Format(time.RFC3339Nano)
	return logEntry{
		LogName:          "projects/moz-fx-ads-prod/logs/stdout",
		Resource:         logEntryResource{Type: "k8s_container", Labels: map[string]string{"cluster_name": "ads-mars-prod-europe-west1", "container_name": "mars", "location": "europe-west1", "namespace_name": "ads", "pod_name": "mars-deployment-7d8f9bc4c5-x7n2j", "project_id": "moz-fx-ads-prod-3f5c"}},
		Timestamp:        now,
		ReceiveTimestamp: now,
		Severity:         "INFO",
		InsertID:         mockUUID(),
		Labels:           map[string]string{"k8s-pod/app": "mars", "k8s-pod/pod-template-hash": "7d8f9bc4c5"},
		JsonPayload:      jp,
	}
}

func gzipBytes(level int, src []byte) []byte {
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, level)
	_, _ = gz.Write(src)
	_ = gz.Close()
	return buf.Bytes()
}

func TestEnvelopeSizes(t *testing.T) {
	cases := []struct {
		name string
		env  envelope
	}{
		{"request_stats_v1", buildRequestStatsV1()},
		{"interaction_v1", buildInteractionV1()},
	}

	fmt.Printf("\n%-22s %-30s %10s\n", "ping_type", "form", "bytes")
	fmt.Println("---------------------------------------------------------------------")
	for _, c := range cases {
		rawJSON, _ := json.Marshal(c.env)
		gzDefault := gzipBytes(gzip.DefaultCompression, rawJSON)
		gzFast := gzipBytes(gzip.BestSpeed, rawJSON)

		wrapped := wrapInLogEntry(c.env)
		wrappedJSON, _ := json.Marshal(wrapped)

		fmt.Printf("%-22s %-30s %10d\n", c.name, "raw envelope (direct PoC)", len(rawJSON))
		fmt.Printf("%-22s %-30s %10d  (ratio %.2fx)\n", c.name, "gzipped (default level 6)", len(gzDefault), float64(len(rawJSON))/float64(len(gzDefault)))
		fmt.Printf("%-22s %-30s %10d  (ratio %.2fx)\n", c.name, "gzipped (BestSpeed level 1)", len(gzFast), float64(len(rawJSON))/float64(len(gzFast)))
		fmt.Printf("%-22s %-30s %10d  (overhead %.2fx)\n", c.name, "Cloud Logging wrapped", len(wrappedJSON), float64(len(wrappedJSON))/float64(len(rawJSON)))
		fmt.Println()
	}

	// Production-comparison summary line: weighted by Feb 2026 volume ratio.
	// request_stats_v1: 2.5B / 2.97B = 84%, interaction_v1: 467M / 2.97B = 16%.
	rs := buildRequestStatsV1()
	ix := buildInteractionV1()
	rsRaw, _ := json.Marshal(rs)
	ixRaw, _ := json.Marshal(ix)
	weightedRaw := 0.84*float64(len(rsRaw)) + 0.16*float64(len(ixRaw))

	rsWrap, _ := json.Marshal(wrapInLogEntry(rs))
	ixWrap, _ := json.Marshal(wrapInLogEntry(ix))
	weightedWrap := 0.84*float64(len(rsWrap)) + 0.16*float64(len(ixWrap))

	rsGz := gzipBytes(gzip.DefaultCompression, rsRaw)
	ixGz := gzipBytes(gzip.DefaultCompression, ixRaw)
	weightedGz := 0.84*float64(len(rsGz)) + 0.16*float64(len(ixGz))

	fmt.Println("Volume-weighted summary (84% request_stats_v1 + 16% interaction_v1):")
	fmt.Printf("  Direct PoC envelope (uncompressed):      %7.0f B\n", weightedRaw)
	fmt.Printf("  Direct PoC envelope + gzip default:      %7.0f B    (%.2fx smaller)\n", weightedGz, weightedRaw/weightedGz)
	fmt.Printf("  Cloud Logging wrapped (current path):    %7.0f B    (%.2fx vs raw direct)\n", weightedWrap, weightedWrap/weightedRaw)
	fmt.Printf("  Wrapped vs gzipped direct:               %.2fx larger\n", weightedWrap/weightedGz)
}

// --- gzip throughput benchmark ---

func BenchmarkGzipDefault(b *testing.B) {
	env := buildRequestStatsV1() // dominant ping type by volume
	data, _ := json.Marshal(env)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		gz, _ := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
		_, _ = gz.Write(data)
		_ = gz.Close()
	}
}

func BenchmarkGzipBestSpeed(b *testing.B) {
	env := buildRequestStatsV1()
	data, _ := json.Marshal(env)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		gz, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		_, _ = gz.Write(data)
		_ = gz.Close()
	}
}

// Realistic publisher pattern: reuse a gzip.Writer instance per goroutine via Reset.
// This is what we'd recommend the template emit, since allocating a new gzip.Writer
// per message has measurable overhead at ~1 KB payloads.
func BenchmarkGzipDefaultWithReset(b *testing.B) {
	env := buildRequestStatsV1()
	data, _ := json.Marshal(env)
	b.SetBytes(int64(len(data)))

	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		gz.Reset(&buf)
		_, _ = gz.Write(data)
		_ = gz.Close()
	}
}
