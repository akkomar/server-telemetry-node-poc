/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// AUTOGENERATED BY glean_parser v14.3.1.dev3+g0d7fb81e. DO NOT EDIT. DO NOT COMMIT.

// This requires `uuid` and `mozlog` libraries to be in the environment
// @types/uuid and mozlog types definitions are required in devDependencies
// for the latter see https://github.com/mozilla/fxa/blob/85bda71cda376c417b8c850ba82aa14252208c3c/types/mozlog/index.d.ts
import { v4 as uuidv4 } from 'uuid';
import mozlog, { Logger } from 'mozlog';

const GLEAN_EVENT_MOZLOG_TYPE = 'glean-server-event';
type LoggerOptions = { app: string; fmt?: 'heka' };
type Event = {
  category: string;
  name: string;
  extra?: Record<string, any>;
  timestamp?: number;
};

let _logger: Logger;

class EventsServerEventLogger {
  _applicationId: string;
  _appDisplayVersion: string;
  _channel: string;
  /**
   * Create EventsServerEventLogger instance.
   *
   * @param {string} applicationId - The application ID.
   * @param {string} appDisplayVersion - The application display version.
   * @param {string} channel - The channel.
   * @param {LoggerOptions} logger_options - The logger options.
   */
  constructor(
    applicationId: string,
    appDisplayVersion: string,
    channel: string,
    logger_options: LoggerOptions
  ) {
    this._applicationId = applicationId;
    this._appDisplayVersion = appDisplayVersion;
    this._channel = channel;

    if (!_logger) {
      // append '-glean' to `logger_options.app` to avoid collision with other loggers and double logging
      logger_options.app = logger_options.app + '-glean';
      // set the format to `heka` so messages are properly ingested and decoded
      logger_options.fmt = 'heka';
      // mozlog types declaration requires a typePrefix to be passed when creating a logger
      // we don't want a typePrefix, so we pass `undefined`
      _logger = mozlog(logger_options)(undefined);
    }
  }
  #record({
    user_agent,
    ip_address,
    identifiers_fxa_account_id,
    event,
  }: {
    user_agent: string;
    ip_address: string;
    identifiers_fxa_account_id: string;
    event: Event;
  }) {
    const now = new Date();
    const timestamp = now.toISOString();
    event.timestamp = now.getTime();
    const eventPayload = {
      metrics: {
        string: {
          'identifiers.fxa_account_id': identifiers_fxa_account_id,
        },
      },
      events: [event],
      ping_info: {
        seq: 0, // this is required, however doesn't seem to be useful in server context
        start_time: timestamp,
        end_time: timestamp,
      },
      // `Unknown` fields below are required in the Glean schema, however they are not useful in server context
      client_info: {
        telemetry_sdk_build: 'glean_parser v14.3.1.dev3+g0d7fb81e',
        first_run_date: 'Unknown',
        os: 'Unknown',
        os_version: 'Unknown',
        architecture: 'Unknown',
        app_build: 'Unknown',
        app_display_version: this._appDisplayVersion,
        app_channel: this._channel,
      },
    };
    const eventPayloadSerialized = JSON.stringify(eventPayload);

    // This is the message structure that Decoder expects: https://github.com/mozilla/gcp-ingestion/pull/2400
    const ping = {
      document_namespace: this._applicationId,
      document_type: 'events',
      document_version: '1',
      document_id: uuidv4(),
      user_agent: user_agent,
      ip_address: ip_address,
      payload: eventPayloadSerialized,
    };

    // this is similar to how FxA currently logs with mozlog: https://github.com/mozilla/fxa/blob/4c5c702a7fcbf6f8c6b1f175e9172cdd21471eac/packages/fxa-auth-server/lib/log.js#L289
    _logger.info(GLEAN_EVENT_MOZLOG_TYPE, ping);
  }
  /**
   * Record and submit a backend_object_update event:
   * Event triggered by the backend to record the change in state of an object (e.g. API requests to the mozilla.social Mastodon server). In the future, we could potentially use this event to track changes in state to core Mastodon objects (e.g. accounts and posts).
   * Event is logged using internal mozlog logger.
   *
   * @param {string} user_agent - The user agent.
   * @param {string} ip_address - The IP address. Will be used to decode Geo
   *                              information and scrubbed at ingestion.
   * @param {string} identifiers_fxa_account_id - The user's FxA account ID, if available..
   * @param {string} object_type - A simple name to describe the object whose state changed. For example, `api_request`..
   * @param {string} object_state - A JSON representation of the latest state of the object..
   * @param {boolean} linking - Indicates the initial linking of the Mozilla account and the third-party account..
   */
  recordBackendObjectUpdate({
    user_agent,
    ip_address,
    identifiers_fxa_account_id,
    object_type,
    object_state,
    linking,
  }: {
    user_agent: string;
    ip_address: string;
    identifiers_fxa_account_id: string;
    object_type: string;
    object_state: string;
    linking: boolean;
  }) {
    const event = {
      category: 'backend',
      name: 'object_update',
      extra: {
        object_type: String(object_type),
        object_state: String(object_state),
        linking: String(linking),
      },
    };
    this.#record({
      user_agent,
      ip_address,
      identifiers_fxa_account_id,
      event,
    });
  }
}

/**
 * Factory function that creates an instance of Glean Server Event Logger to
 * record `events` ping events.
 * @param {string} applicationId - The application ID.
 * @param {string} appDisplayVersion - The application display version.
 * @param {string} channel - The channel.
 * @param {Object} logger_options - The logger options.
 * @returns {EventsServerEventLogger} An instance of EventsServerEventLogger.
 */
export const createEventsServerEventLogger = function ({
  applicationId,
  appDisplayVersion,
  channel,
  logger_options,
}: {
  applicationId: string;
  appDisplayVersion: string;
  channel: string;
  logger_options: LoggerOptions;
}) {
  return new EventsServerEventLogger(
    applicationId,
    appDisplayVersion,
    channel,
    logger_options
  );
};
