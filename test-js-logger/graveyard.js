const express = require('express')

const app = express()
// create your mozlog instance
const mozlog = require('mozlog')({
  app: 'fxa-oauth-server',
  level: 'verbose', //default is INFO
  fmt: 'heka', //default is 'heka'
  uncaught: 'exit', // default is 'log', also available as 'ignore'
  debug: true, //default is false
  stream: process.stdout //default is process.stdout
});
const log = mozlog();

function logBadPing() {
  //log ping without client_info, it should be rejected at validation
  let eventPayload = `{
    "metrics": {
      "string": {
        "event.name": "reg_submit_success",
        "account.user_id_sha256": "",
        "relying_party.oauth_client_id": "",
        "relying_party.service": "sync",
        "session.device_type": "desktop",
        "session.entrypoint": "",
        "session.flow_id": "5d1eaf933f521cb2a15af909c813673ada8485d6ace8e806c57148cd7f13b30c",
        "utm.campaign": "",
        "utm.content": "",
        "utm.medium": "",
        "utm.source": "",
        "utm.term": ""
      }
    },
    "ping_info": {
      "seq": 2,
      "start_time": "2023-06-22T11:28-05:00",
      "end_time": "2023-06-22T11:28-05:00"
    }
  }`;
  eventPayload = JSON.stringify(JSON.parse(eventPayload));
  let ping = {
    "document_namespace": "accounts-backend",
    "document_type": "accounts-events",
    "document_version": "1",
    "document_id": "dd99db96-941d-4894-bb97-7a2bcd65bbf5",
    "payload": eventPayload
  }
  log.info('glean-server-event', ping);
}

app.get('/', (req, res) => {
  res.send('Hello World!');
})

app.listen(3000, () => {
  console.log('Example app listening on port 3000!');
})

setInterval(() => {
    console.log('Logging something every second');
    const timestamp = new Date().toISOString();


    let eventPayload = `{
      "metrics": {
        "string": {
          "event.name": "reg_submit_success",
          "account.user_id_sha256": "",
          "relying_party.oauth_client_id": "",
          "relying_party.service": "sync",
          "session.device_type": "desktop",
          "session.entrypoint": "",
          "session.flow_id": "5d1eaf933f521cb2a15af909c813673ada8485d6ace8e806c57148cd7f13b30c",
          "utm.campaign": "",
          "utm.content": "",
          "utm.medium": "",
          "utm.source": "",
          "utm.term": ""
        }
      },
      "ping_info": {
        "seq": 2,
        "start_time": "2023-06-22T11:28-05:00",
        "end_time": "2023-06-22T11:28-05:00"
      },
      "client_info": {
        "telemetry_sdk_build": "1.4.0",
        "client_id": "73f5ae86-90f8-46a3-9ccc-bca902a175bf",
        "first_run_date": "2023-06-22-05:00",
        "os": "Darwin",
        "os_version": "Unknown",
        "architecture": "Unknown",
        "locale": "en-US",
        "app_build": "Unknown",
        "app_display_version": "0.0.0",
        "app_channel": "development"
      }
    }`;
    eventPayload = JSON.stringify(JSON.parse(eventPayload));
    // this outputs escaped payload and is similar to how FxA logs events right now: https://github.com/mozilla/fxa/blob/6d65d20d5cdd7e70f29f1c6e3d44185f0d06f117/packages/fxa-auth-server/lib/log.js#L289
    // {"Timestamp":1687448048064000000,"Logger":"fxa-oauth-server","Type":"glean-server-event","Severity":6,"Pid":33056,"EnvVersion":"2.0","Fields":{"payload":"{\"timestamp\": \"2023-06-22T15:34:08.058Z\", \"event_name\": \"test\"}"}}
    let ping = {
      "document_namespace": "accounts-frontend", // using `accounts-frontend` here for testing the Decoder because we don't have the backend schema yet
      "document_type": "accounts-events",
      "document_version": "1",
      "document_id": "dd99db96-941d-4894-bb97-7a2bcd65bbf5",
      "payload": eventPayload
    }
    log.info('glean-server-event', ping);
    logBadPing();


    // this outputs payload as a nested JSON so it will be automatically parsed by BQ sink
    // {"Timestamp":1687448048078000000,"Logger":"fxa-oauth-server","Type":"glean-server-event-from-dict","Severity":6,"Pid":33056,"EnvVersion":"2.0","Fields":{"timestamp":"2023-06-22T15:34:08.058Z","event_name":"test"}}
    const dict = {timestamp: timestamp, event_name: 'test'};
    log.info('glean-server-event-from-dict', dict);

    // this outputs the string in an `msg` field with JSON part escaped
    // {"Timestamp":1687448048080000000,"Logger":"fxa-oauth-server","Type":"glean-server-event-with-prefix","Severity":6,"Pid":33056,"EnvVersion":"2.0","Fields":{"msg":"some-string {\"timestamp\": \"2023-06-22T15:34:08.058Z\", \"event_name\": \"test\"}"}}
    log.info('glean-server-event-with-prefix', `some-string {"timestamp": "${timestamp}", "event_name": "test"}`);

    console.log('non-event log line')


  }, 1000)
