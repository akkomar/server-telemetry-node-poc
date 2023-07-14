/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// AUTOGENERATED BY glean_parser v7.2.2.dev8+g91d4c811. DO NOT EDIT. DO NOT COMMIT.

// This requires `uuid` and `mozlog` libraries to be in the environment
const uuidv4 = require('uuid').v4;
const mozlog = require('mozlog');

const GLEAN_EVENT_MOZLOG_TYPE = 'glean-server-event';

var _logger;

class AccountsEventsServerEvent {
 /**
   * Create AccountsEventsServerEvent instance.
   *
   * @param {string} applicationId - The application ID.
   * @param {string} appDisplayVersion - The application display version.
   * @param {string} channel - The channel.
   */
  constructor(applicationId, appDisplayVersion, channel, logger_options) {
    this._applicationId = applicationId;
    this._appDisplayVersion = appDisplayVersion;
    this._channel = channel;

    if (!_logger) {
      // append '-glean' to `logger_options.app` to avoid collision with other loggers and double logging
      logger_options.app = logger_options.app + '-glean';
      // set the format to `heka` so messages are properly ingested and decoded
      logger_options.fmt = 'heka';
      _logger = mozlog(logger_options)();
    }
    this._log = _logger;
  }
  /**
   * Record and submit a server event object.
   * Event is logged using internal mozlog logger.
   *
   * @param {string} account_user_id_sha256 - A hex string of a sha256 hash of the account's uid.
   * @param {string} event_name - The name of the event.
   * @param {string} relying_party_service - The service name of the relying party.
   */
  record({
    account_user_id_sha256,
    event_name,
    relying_party_service,
  }) {
    let timestamp = new Date().toISOString();
    let eventPayload = {
      metrics: {
        string: {
          'account.user_id_sha256': account_user_id_sha256,
          'event.name': event_name,
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
        telemetry_sdk_build: 'glean_parser v7.2.2.dev8+g91d4c811',
        first_run_date: 'Unknown',
        os: 'Unknown',
        os_version: 'Unknown',
        architecture: 'Unknown',
        app_build: 'Unknown',
        app_display_version: this._appDisplayVersion,
        app_channel: this._channel,
      },
    };
    eventPayload = JSON.stringify(eventPayload);

    // This is the message structure that Decoder expects: https://github.com/mozilla/gcp-ingestion/pull/2400
    let ping = {
      document_namespace: this._applicationId,
      document_type: 'accounts-events',
      document_version: '1',
      document_id: uuidv4(),
      payload: eventPayload,
    };

    // this is similar to how FxA currently logs with mozlog: https://github.com/mozilla/fxa/blob/4c5c702a7fcbf6f8c6b1f175e9172cdd21471eac/packages/fxa-auth-server/lib/log.js#L289
    this._log.info(GLEAN_EVENT_MOZLOG_TYPE, ping);
  }
}

const createAccountsEventsEventFn = function ({
  applicationId,
  appDisplayVersion,
  channel,
  logger_options
}) {
  return new AccountsEventsServerEvent(
    applicationId,
    appDisplayVersion,
    channel,
    logger_options
  );
};

module.exports = {
    createAccountsEventsEventFn,
};
