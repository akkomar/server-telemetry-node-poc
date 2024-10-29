pub mod glean {
    pub mod server_events;
}

use crate::glean::server_events::{
    BackendObjectUpdateEvent, EventsPing, GleanEventsLogger, RequestInfo,
};
use std::thread::sleep;
use std::time::Duration;

fn main() {
    let logger = GleanEventsLogger {
        app_id: "test-rust-logger".to_string(),
        app_display_version: "1.0.0".to_string(),
        app_channel: "development".to_string(),
    };

    // Simulate events in a loop
    loop {
        logger.record_events_ping(
            &RequestInfo {
                user_agent: "Mozilla/5.0".to_string(),
                ip_address: "192.168.1.1".to_string(),
            },
            &EventsPing {
                identifiers_fxa_account_id: "1234".to_string(),
                event: Some(Box::new(BackendObjectUpdateEvent {
                    object_type: "your_object_type".to_string(),
                    object_state: "your_object_state".to_string(),
                    linking: true,
                })),
            },
        );

        sleep(Duration::from_secs(1)); // Adjust the duration as needed
    }
}
