import time
from glean.server_events import create_events_server_event_logger, EventsServerEventLogger

def main():

    logger = create_events_server_event_logger(
        application_id="accounts_backend",
        app_display_version="0.0.1",
        channel="nightly",
    )

    while True:
        print("test py logger")

        logger.record_email_generate_mask(
            user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36 Edg/115.0.1901.188",
            ip_address="2a02:a311:803c:6300:4074:5cf2:91ac:d546",
            mozilla_accounts_id="test-py-project",
            is_random_mask="True",
            created_by_api="True",
            has_generated_for="True",
        )

        time.sleep(1)

if __name__ == "__main__":
    main()
