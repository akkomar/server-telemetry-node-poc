# WiP notes
## Decoding
We have two options: decoder in batch mode reading from BQ or from Pub/Sub
### Decoder reading from BQ
Query to prepare a Decoder input table:
```sql
CREATE OR REPLACE TABLE
  glean_server_event.decoder_input AS
SELECT
  timestamp AS submission_timestamp,
  jsonPayload.fields.document_id,
  -- jsonPayload.fields.document_namespace,
  "accounts-frontend" AS document_namespace,
  jsonPayload.fields.document_type,
  jsonPayload.fields.document_version,
  CAST(jsonPayload.fields.payload AS BYTES) AS payload,
FROM
  `akomar-server-telemetry-poc.glean_server_event.stdout`
WHERE
  timestamp >= "2023-06-23 16:00:00.000000 UTC"
  AND jsonPayload.type='glean-server-event'
LIMIT
  100
```

Create a table for decoded pings:
```sql
CREATE OR REPLACE TABLE
  accounts_frontend_live.accounts_events_v1 LIKE moz-fx-data-shared-prod.accounts_frontend_live.accounts_events_v1
```

Create table an error table:
```sql
CREATE OR REPLACE TABLE
  payload_bytes_error.structured (
    args STRING,
    client_id STRING,
    content_length STRING,
    date STRING,
    dnt STRING,
    document_namespace STRING,
    document_type STRING,
    document_version STRING,
    error_message STRING,
    error_type STRING,
    exception_class STRING,
    geo_city STRING,
    geo_country STRING,
    geo_subdivision1 STRING,
    geo_subdivision2 STRING,
    host STRING,
    input STRING,
    input_type STRING,
    job_name STRING,
    method STRING,
    payload BYTES,
    protocol STRING,
    remote_addr STRING,
    stack_trace STRING,
    stack_trace_cause_1 STRING,
    stack_trace_cause_2 STRING,
    stack_trace_cause_3 STRING,
    stack_trace_cause_4 STRING,
    stack_trace_cause_5 STRING,
    submission_timestamp TIMESTAMP,
    uri STRING,
    user_agent STRING,
    user_agent_browser STRING,
    user_agent_os STRING,
    user_agent_version STRING,
    x_debug_id STRING,
    x_forwarded_for STRING,
    x_pingsender_version STRING,
    x_pipeline_proxy STRING,
    isp_name STRING,
    isp_organization STRING,
    x_source_tags STRING,
    x_telemetry_agent STRING,
    x_foxsec_ip_reputation STRING,
    x_lb_tags STRING )
PARTITION BY DATE(submission_timestamp)
CLUSTER BY submission_timestamp
```

Run the Decoder:
```sh
cd ../gcp-ingestion
../server-telemetry-node-poc/ingestion-beam-decoder.sh
```

Check the decoded data:
```sql
SELECT
  *
FROM
  `akomar-server-telemetry-poc.accounts_frontend_live.accounts_events_v1`
WHERE
  DATE(submission_timestamp)='2023-06-24'
LIMIT
  1000
```

### Decoder reading from Pub/Sub
Create a Pub/Sub sink:
```
gcloud logging sinks create glean-event-pubsub-sink pubsub.googleapis.com/projects/akomar-server-telemetry-poc/topics/glean-server-event --log-filter='jsonPayload.Type=~"glean-server-event*"'
```
