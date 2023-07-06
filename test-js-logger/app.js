const express = require('express');

const glean_server_events = require('./glean-server/server_events');

const app = express();
// create your mozlog instance
const mozlog = require('mozlog')({
  app: 'fxa-oauth-server',
  level: 'verbose', //default is INFO
  fmt: 'heka', //default is 'heka'
  uncaught: 'exit', // default is 'log', also available as 'ignore'
  debug: true, //default is false
  stream: process.stdout, //default is process.stdout
});
const log = mozlog();

function logBadPing() {
  //log ping without client_info, it should be rejected at validation
  let eventPayload = {
    metrics: {
      string: {
        'event.name': 'reg_submit_success',
        'account.user_id_sha256': '',
      },
    },
    ping_info: {
      seq: 2,
      start_time: '2023-06-22T11:28-05:00',
      end_time: '2023-06-22T11:28-05:00',
    },
  };
  eventPayload = JSON.stringify(eventPayload);
  let ping = {
    document_namespace: 'accounts-backend',
    document_type: 'accounts-events',
    document_version: '1',
    document_id: 'dd99db96-941d-4894-bb97-7a2bcd65bbf5',
    payload: eventPayload,
  };
  log.info('glean-server-event', ping);
}

app.get('/', (req, res) => {
  res.send('Hello World!');
});

app.listen(3000, () => {
  console.log('Example app listening on port 3000!');
});

setInterval(() => {
  //============================================================================
  // This is the new way to log server events with Glean
  let glean_event = glean_server_events.event.record({
    account_user_id_sha256: 'abc',
    relying_party_oauth_client_id: '123',
    relying_party_service: 'sync',
  });
  // this is similar to how FxA currently logs with mozlog: https://github.com/mozilla/fxa/blob/4c5c702a7fcbf6f8c6b1f175e9172cdd21471eac/packages/fxa-auth-server/lib/log.js#L289
  // it might make sense to bake the following line in the `record` function called above to make simpler to use
  log.info(glean_server_events.GLEAN_EVENT_MOZLOG_TYPE, glean_event);
  //============================================================================

  // log some other stuff to test validation
  logBadPing();
  console.log('non-event log line');
}, 1000);
