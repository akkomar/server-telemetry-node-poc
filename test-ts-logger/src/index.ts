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
  // TODO: generate the code under `glean-server/server_events.ts` using glean_parser

  // First create an event instance providing a set of parameters that are constant during the lifetime of the application
  let event = createAccountsEventsEvent({
    applicationId: 'accounts-frontend',
    appDisplayVersion: '0.0.1',
    channel: 'development',
    logger_options: {
      app: 'fxa-oauth-server',
      stream: process.stdout, //default is process.stdout
    },
  });

  // Then log events using the event instance
  event.record({
    event_name: 'reg_view',
    account_user_id_sha256: 'abc',
    relying_party_service: 'sync',
  });
  //============================================================================
}, 1000);