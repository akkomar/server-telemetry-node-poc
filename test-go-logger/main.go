package main

import (
	"test-go-logger/glean"
	"time"
)

func main() {
    // This uses code generated by glean_parser that is under active development
    // see https://github.com/akkomar/glean_parser/tree/go_server_support
    // (based on :dmueller's PR from a fork: https://github.com/mozilla/glean_parser/pull/645)
    // To generate the code under `glean/server_events.go`, check out the branch above and run:
    // glean_parser translate tests/data/server_metrics_with_event.yaml -f go_server -o ../server-telemetry-node-poc/test-go-logger/glean

	for {
		logger := glean.GleanEventsLogger{
			AppID:             "test-go-logger",
			AppDisplayVersion: "0.0.1",
			AppChannel:        "nightly",
		}

		requestInfo := glean.RequestInfo{
		}

		params := glean.EventBackendObjectUpdate{
			IdentifiersFxaAccountId: "your_account_id",
			ObjectType:              "your_object_type",
			ObjectState:             "your_object_state",
		}

		logger.RecordEventBackendObjectUpdate(requestInfo, params)

		time.Sleep(time.Second)
	}
}
