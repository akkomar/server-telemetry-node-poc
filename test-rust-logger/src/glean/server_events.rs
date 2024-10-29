use serde::Serialize;
use serde_json;
use uuid::Uuid;
use std::time::{SystemTime, UNIX_EPOCH};

const GLEAN_EVENT_MOZLOG_TYPE: &str = "glean-server-event";

#[derive(Serialize)]
pub struct GleanEventsLogger {
    pub app_id: String,
    pub app_display_version: String,
    pub app_channel: String,
}

#[derive(Serialize)]
pub struct RequestInfo {
    pub user_agent: String,
    pub ip_address: String,
}

impl Default for RequestInfo {
    fn default() -> Self {
        RequestInfo {
            user_agent: "".to_string(),
            ip_address: "".to_string(),
        }
    }
}

#[derive(Serialize)]
struct ClientInfo {
    telemetry_sdk_build: String,
    first_run_date: String,
    os: String,
    os_version: String,
    architecture: String,
    app_build: String,
    app_display_version: String,
    app_channel: String,
}

#[derive(Serialize)]
struct PingInfo {
    seq: i32,
    start_time: String,
    end_time: String,
}

#[derive(Serialize)]
struct Ping {
    document_namespace: String,
    document_type: String,
    document_version: String,
    document_id: String,
    user_agent: Option<String>,
    ip_address: Option<String>,
    payload: String,
}

type Metrics = std::collections::HashMap<String, std::collections::HashMap<String, serde_json::Value>>;

#[derive(Serialize)]
struct PingPayload {
    client_info: ClientInfo,
    ping_info: PingInfo,
    metrics: Metrics,
    events: Vec<GleanEvent>,
}

#[derive(Serialize)]
pub struct GleanEvent {
    category: String,
    name: String,
    timestamp: i64,
    extra: std::collections::HashMap<String, String>,
}

fn new_glean_event(category: &str, name: &str, extra: std::collections::HashMap<String, String>) -> GleanEvent {
    GleanEvent {
        category: category.to_string(),
        name: name.to_string(),
        timestamp: SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_millis() as i64,
        extra,
    }
}

#[derive(Serialize)]
struct LogEnvelope {
    timestamp: String,
    logger: String,
    #[serde(rename = "type")]
    log_type: String,
    fields: Ping,
}

impl GleanEventsLogger {
    fn create_client_info(&self) -> ClientInfo {
        ClientInfo {
            telemetry_sdk_build: "glean_parser v15.0.2.dev17+g81fec69a".to_string(),
            first_run_date: "n/a".to_string(),
            os: "n/a".to_string(),
            os_version: "n/a".to_string(),
            architecture: "n/a".to_string(),
            app_build: "n/a".to_string(),
            app_display_version: self.app_display_version.clone(),
            app_channel: self.app_channel.clone(),
        }
    }

    fn create_ping_info() -> PingInfo {
        let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap();
        let now_str = format!("{:?}", now);
        PingInfo {
            seq: 0,
            start_time: now_str.clone(),
            end_time: now_str,
        }
    }

    fn create_ping(&self, document_type: &str, config: &RequestInfo, payload: &PingPayload) -> Ping {
        let payload_json = serde_json::to_string(payload).expect("Unable to marshal payload to json");
        let document_id = Uuid::new_v4().to_string();
        Ping {
            document_namespace: self.app_id.clone(),
            document_type: document_type.to_string(),
            document_version: "1".to_string(),
            document_id,
            user_agent: Some(config.user_agent.clone()),
            ip_address: Some(config.ip_address.clone()),
            payload: payload_json,
        }
    }

    fn record(&self, document_type: &str, request_info: &RequestInfo, metrics: Metrics, events: Vec<GleanEvent>) {
        let telemetry_payload = PingPayload {
            client_info: self.create_client_info(),
            ping_info: GleanEventsLogger::create_ping_info(),
            metrics,
            events,
        };

        let ping = self.create_ping(document_type, request_info, &telemetry_payload);

        let envelope = LogEnvelope {
            timestamp: SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_nanos().to_string(),
            logger: "glean".to_string(),
            log_type: GLEAN_EVENT_MOZLOG_TYPE.to_string(),
            fields: ping,
        };

        let envelope_json = serde_json::to_string(&envelope).expect("Unable to marshal log envelope to json");
        println!("{}", envelope_json);
    }
}

// Code above is static irrespective of metrics definitions.
// Code below is generated based on metrics.yaml:

// pings:
pub struct EventsPing {
    pub identifiers_fxa_account_id: String,
    pub event: Option<Box<dyn EventsPingEvent>>,
}

// marker trait for events per ping
pub trait EventsPingEvent {
    fn glean_event(&self) -> GleanEvent;
}

// specific events
pub struct BackendObjectUpdateEvent {
    pub object_type: String,
    pub object_state: String,
    pub linking: bool,
}

impl EventsPingEvent for BackendObjectUpdateEvent {
    fn glean_event(&self) -> GleanEvent {
        let mut extra = std::collections::HashMap::new();
        extra.insert("object_type".to_string(), self.object_type.clone());
        extra.insert("object_state".to_string(), self.object_state.clone());
        extra.insert("linking".to_string(), self.linking.to_string());

        new_glean_event("backend", "object_update", extra)
    }
}

// logger, with specific record method for each ping
impl GleanEventsLogger {
    pub fn record_events_ping(&self, request_info: &RequestInfo, params: &EventsPing) {
        let mut metrics = Metrics::new();
        let mut string_map = std::collections::HashMap::new();
        string_map.insert("identifiers.fxa_account_id".to_string(), serde_json::Value::String(params.identifiers_fxa_account_id.clone()));
        metrics.insert("string".to_string(), string_map);

        let mut events = Vec::new();
        if let Some(event) = &params.event {
            events.push(event.glean_event());
        }

        self.record("events", request_info, metrics, events);
    }
}
