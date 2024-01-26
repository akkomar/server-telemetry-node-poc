import time
from glean.server_events import create_events_server_event_logger, EventsServerEventLogger

def main():

    # Glean logging code generated via:
    # glean_parser translate tests/data/server_metrics_with_event.yaml -f python_server -o ../server-telemetry-node-poc/test-py-logger/glean

    logger = create_events_server_event_logger(
        application_id="accounts_backend",
        app_display_version="0.0.1",
        channel="nightly",
    )

    while True:
        print("test py logger")

        logger.record_backend_object_update(
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36 Edg/115.0.1901.188",
            ip_address="2a02:a311:803c:6300:4074:5cf2:91ac:d546",
            identifiers_fxa_account_id="test-py-project",
            object_type="some_type",
		    object_state="some_state",
        )

        time.sleep(1)

if __name__ == "__main__":
    main()
