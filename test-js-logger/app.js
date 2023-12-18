import express from 'express';
import mozlog from 'mozlog';

import { createAccountsEventsEvent } from './glean-server/server_events.js';
import { createEventsServerEventLogger } from './glean-server-event-metric/server_events.js';

const app = express();
// create your mozlog instance
const log = mozlog({
  app: 'fxa-oauth-server',
  level: 'verbose', //default is INFO
  fmt: 'heka', //default is 'heka'
  uncaught: 'exit', // default is 'log', also available as 'ignore'
  debug: true, //default is false
  stream: process.stdout, //default is process.stdout
})();

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
    document_namespace: 'accounts-frontend',
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
  // New way to log server events with Glean
  // This uses code generated by glean_parser that is under active development
  // see https://github.com/akkomar/glean_parser/tree/server_events
  // To generate the code under `glean-server/server_events.js`, check out the branch above and run:
  // glean_parser translate ../server-telemetry-node-poc/test-js-logger/metrics/metrics.yaml ../server-telemetry-node-poc/test-js-logger/metrics/pings.yaml -f javascript_server -o ../server-telemetry-node-poc/test-js-logger/glean-server

  // First create an event instance providing a set of parameters that are constant during the lifetime of the application
  // let event = createAccountsEventsEvent({
  //   applicationId: 'accounts-frontend',
  //   appDisplayVersion: '0.0.1',
  //   channel: 'development',
  //   logger_options: {
  //     app: 'fxa-oauth-server',
  //     stream: process.stdout, //default is process.stdout
  //   },
  // });

  // // Then log events using the event instance
  // event.record({
  //   user_agent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36 Edg/115.0.1901.188',
  //   ip_address: '2a02:a311:803c:6300:4074:5cf2:91ac:d546',
  //   event_name: 'reg_view',
  //   account_user_id_sha256: 'abc',
  //   relying_party_oauth_client_id: '123',
  //   relying_party_service: 'sync',
  // });

  // Logging with event metric type - new, recommended way
  // generated via:
  // glean_parser translate tests/data/ruby_server_pings.yaml tests/data/ruby_server_metrics.yaml -f javascript_server -o ../server-telemetry-node-poc/test-js-logger/glean-server-event-metric

  // First create an event instance providing a set of parameters that are constant during the lifetime of the application
  let gleanEventLogger = createEventsServerEventLogger({
    applicationId: 'accounts-frontend',
    appDisplayVersion: '0.0.1',
    channel: 'development',
    logger_options: {
      app: 'fxa-oauth-server',
      stream: process.stdout, //default is process.stdout
    },
  });
  // Then log events using the event logger instance
  gleanEventLogger.recordBackendObjectUpdate({
    user_agent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36 Edg/115.0.1901.188',
    ip_address: '2a02:a311:803c:6300:4074:5cf2:91ac:d546',
    identifiers_fxa_account_id: 'abc',
    object_type: 'unknown',
    object_state: 'great',
  });
  //============================================================================

  // log some other stuff to test validation
  logBadPing();
  console.log('non-event log line');
}, 1000);
