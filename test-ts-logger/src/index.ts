import express from 'express';
import { createAccountsEventsEvent } from './glean-server/server_events';

const app = express();
const port = 3000;

app.get('/', (req, res) => {
  res.send('Hello World!');
});

app.listen(port, () => {
  console.log(`Server listening on port ${port}.`);
});

setInterval(() => {
  //============================================================================
  // New way to log server events with Glean
  // This uses code generated by glean_parser that is under active development
  // see https://github.com/akkomar/glean_parser/tree/server_events
  // To generate the code under `glean-server/server_events.ts`, check out the branch above and run:
  // glean_parser translate ../server-telemetry-node-poc/test-js-logger/metrics/metrics.yaml ../server-telemetry-node-poc/test-js-logger/metrics/pings.yaml -f typescript_server -o ../server-telemetry-node-poc/test-ts-logger/src/glean-server

  // First create an event instance providing a set of parameters that are constant during the lifetime of the application
  let event = createAccountsEventsEvent({
    applicationId: 'accounts-frontend',
    appDisplayVersion: '0.0.1',
    channel: 'development',
    logger_options: {
      app: 'fxa-oauth-server',
      // stream: process.stdout, //default is process.stdout
    },
  });

  // Then log events using the event instance
  event.record({
    user_agent: 'Mozilla/5.0 (X11; Linux x86_64; rv:85.0) Gecko/20100101 Firefox/85.0',
    ip_address: '2a02:a311:803c:6300:4074:5cf2:91ac:d546',
    event_name: 'reg_view',
    account_user_id_sha256: 'abc',
    relying_party_service: 'sync',
  });
  //============================================================================
}, 1000);
