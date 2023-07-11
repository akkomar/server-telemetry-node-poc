// This will be generated by glean-parser
// FxA frontend uses "pings as events" approach

// This requires `uuid` library to be in the environment
const uuidv4 = require('uuid').v4;

class AccountsEventsServerEvent {
  /**
   * Record a server event object.
   * It must be logged with mozlog with the type 'glean-server-event'
   * in order to be correctly ingested.
   *
   * @returns {Object} A ping object containing information about the recorded event.
   */
  record({
    applicationId,
    appDisplayVersion,
    channel,
    event_name,
    account_user_id_sha256,
    relying_party_oauth_client_id,
    relying_party_service,
  }) {
    let timestamp = new Date().toISOString();
    let eventPayload = {
      metrics: {
        string: {
          'event.name': event_name,
          'account.user_id_sha256': account_user_id_sha256,
          'relying_party.oauth_client_id': relying_party_oauth_client_id,
          'relying_party.service': relying_party_service,
        },
      },
      ping_info: {
        seq: 0, // this is required, however doesn't seem to be useful in server context
        start_time: timestamp,
        end_time: timestamp,
      },
      // `Unknown` fields below are required in the Glean schema, however they are not useful in server context
      client_info: {
        telemetry_sdk_build: 'glean-parser v. X.Y.Z',
        first_run_date: 'Unknown',
        os: 'Unknown',
        os_version: 'Unknown',
        architecture: 'Unknown',
        app_build: 'Unknown',
        app_display_version: appDisplayVersion,
        app_channel: channel,
      },
    };
    eventPayload = JSON.stringify(eventPayload);
    // this outputs escaped payload and is similar to how FxA logs events right now: https://github.com/mozilla/fxa/blob/6d65d20d5cdd7e70f29f1c6e3d44185f0d06f117/packages/fxa-auth-server/lib/log.js#L289
    // {"Timestamp":1687448048064000000,"Logger":"fxa-oauth-server","Type":"glean-server-event","Severity":6,"Pid":33056,"EnvVersion":"2.0","Fields":{"payload":"{\"timestamp\": \"2023-06-22T15:34:08.058Z\", \"event_name\": \"test\"}"}}
    let ping = {
      document_namespace: applicationId,
      document_type: 'accounts-events',
      document_version: '1',
      document_id: uuidv4(),
      payload: eventPayload,
    };

    return ping;
  }
}

module.exports = {
  accountsEventsEvent: new AccountsEventsServerEvent(),
  GLEAN_EVENT_MOZLOG_TYPE: 'glean-server-event',
};
