package main

import (
	"test-go-logger/glean"
	"time"
)

func main() {
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
