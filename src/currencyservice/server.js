/*
 * Copyright 2018 Google LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

const pino = require('pino');
const logger = pino({
  name: 'currencyservice-server',
  messageKey: 'message',
  formatters: {
    level(logLevelString) {
      return { severity: logLevelString };
    },
  },
});

const path = require('path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');

const MAIN_PROTO_PATH = path.join(__dirname, './proto/demo.proto');
const HEALTH_PROTO_PATH = path.join(
  __dirname,
  './proto/grpc/health/v1/health.proto',
);

const PORT = process.env.PORT;

const shopProto = _loadProto(MAIN_PROTO_PATH).hipstershop;
const healthProto = _loadProto(HEALTH_PROTO_PATH).grpc.health.v1;

function _loadProto(path) {
  const packageDefinition = protoLoader.loadSync(path, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  return grpc.loadPackageDefinition(packageDefinition);
}

// Exchange rates against the euro, from European Central Bank public data.
function _getCurrencyData(callback) {
  const data = require('./data/currency_conversion.json');
  callback(data);
}

// Moves the fraction of units into nanos, and whole units out of nanos.
function _carry(amount) {
  const fractionSize = Math.pow(10, 9);
  amount.nanos += (amount.units % 1) * fractionSize;
  amount.units =
    Math.floor(amount.units) + Math.floor(amount.nanos / fractionSize);
  amount.nanos = amount.nanos % fractionSize;
  return amount;
}

function getSupportedCurrencies(call, callback) {
  logger.info('Getting supported currencies..\.');
  _getCurrencyData((data) => {
    callback(null, { currency_codes: Object.keys(data) });
  });
}

function convert(call, callback) {
  try {
    _getCurrencyData((data) => {
      const request = call.request;

      // An unknown code would turn the amount into NaN, which is sent as 0.
      for (const code of [request.from?.currency_code, request.to_code]) {
        if (!Object.hasOwn(data, code ?? '')) {
          logger.warn(
            `conversion request rejected: unsupported currency code "${code ?? ''}"`,
          );
          callback({
            code: grpc.status.INVALID_ARGUMENT,
            message: `unsupported currency code "${code ?? ''}"`,
          });
          return;
        }
      }

      // Convert: from_currency --> EUR
      const from = request.from;
      const euros = _carry({
        units: from.units / data[from.currency_code],
        nanos: from.nanos / data[from.currency_code],
      });

      euros.nanos = Math.round(euros.nanos);

      // Convert: EUR --> to_currency
      const result = _carry({
        units: euros.units * data[request.to_code],
        nanos: euros.nanos * data[request.to_code],
      });

      result.units = Math.floor(result.units);
      result.nanos = Math.floor(result.nanos);
      result.currency_code = request.to_code;

      logger.info(`conversion request successful`);
      callback(null, result);
    });
  } catch (err) {
    logger.error(`conversion request failed: ${err}`);
    callback({ code: grpc.status.INTERNAL, message: err.message });
  }
}

function check(call, callback) {
  callback(null, { status: 'SERVING' });
}

// How long in-flight calls get to finish on SIGTERM: less than the 30 s
// Kubernetes waits by default before sending SIGKILL.
const SHUTDOWN_TIMEOUT_MS = 10000;

/**
 * Stops the server, letting in-flight calls finish but no longer than
 * timeoutMs: then it cuts the rest. Resolves once the server is stopped.
 */
function gracefulShutdown(server, timeoutMs = SHUTDOWN_TIMEOUT_MS) {
  return new Promise((resolve) => {
    const timer = setTimeout(() => {
      logger.warn(`calls still in flight after ${timeoutMs} ms, cutting them`);
      server.forceShutdown();
      resolve();
    }, timeoutMs);
    server.tryShutdown(() => {
      clearTimeout(timer);
      resolve();
    });
  });
}

/**
 * Stops the server gracefully on SIGTERM (what Kubernetes sends to delete a
 * pod) and SIGINT, then exits. Node, as PID 1 in the container, would
 * otherwise ignore SIGTERM until the SIGKILL sent 30 s later.
 */
function stopOnSignals(server) {
  for (const signal of ['SIGTERM', 'SIGINT']) {
    process.once(signal, async () => {
      logger.info(`received ${signal}, shutting down`);
      await gracefulShutdown(server);
      process.exit(0);
    });
  }
}

function main() {
  logger.info(`Starting gRPC server on port ${PORT}...`);
  const server = new grpc.Server();
  server.addService(shopProto.CurrencyService.service, {
    getSupportedCurrencies,
    convert,
  });
  server.addService(healthProto.Health.service, { check });

  server.bindAsync(
    `[::]:${PORT}`,
    grpc.ServerCredentials.createInsecure(),
    function () {
      logger.info(`CurrencyService gRPC server started on port ${PORT}`);
    },
  );
  stopOnSignals(server);
}

module.exports = {
  _carry,
  convert,
  getSupportedCurrencies,
  check,
  gracefulShutdown,
  stopOnSignals,
};

if (require.main === module) {
  main();
}
