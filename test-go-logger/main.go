package main

import (
	"fmt"
	"test-go-logger/glean"
	"time"
)

func main() {
	// This uses code generated by [glean_parser](https://github.com/mozilla/glean_parser)
	// To generate the code under `glean/server_events.go`, check out the main branch of glean_parser and run:
	// glean_parser translate tests/data/server_metrics_with_event.yaml -f go_server -o ../server-telemetry-node-poc/test-go-logger/glean

	for {
		logger := glean.GleanEventsLogger{
			AppID:             "accounts_backend",
			AppDisplayVersion: "0.0.1",
			AppChannel:        "nightly",
		}

		requestInfo := glean.RequestInfo{}

		params := glean.EventBackendObjectUpdate{
			IdentifiersFxaAccountId: "your_account_id",
			ObjectType:              "your_object_type",
			ObjectState:             "your_object_state",
		}

		logger.RecordEventBackendObjectUpdate(requestInfo, params)

		fmt.Println("{\"Type\":\"glean-server-event\",\"Fields\":{}}")

		time.Sleep(time.Second)
	}
}
