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

const path = require('node:path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const logger = require('./logger')('currencyservice-server');

// Exchange rates against the euro, from European Central Bank public data.
const ratesPerEuro = require('./data/currency_conversion.json');

// Below the 30 s Kubernetes waits before sending SIGKILL.
const SHUTDOWN_TIMEOUT_MS = 10000;

const NANOS_PER_UNIT = 1e9;

function loadProto(file) {
  const packageDefinition = protoLoader.loadSync(
    path.join(__dirname, 'proto', file),
    {
      keepCase: true,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    },
  );
  return grpc.loadPackageDefinition(packageDefinition);
}

const shopProto = loadProto('demo.proto').hipstershop;
const healthProto = loadProto('grpc/health/v1/health.proto').grpc.health.v1;

// Truncates towards zero so units and nanos keep the same sign (google.type.Money).
function carry(amount) {
  const nanos = amount.nanos + (amount.units % 1) * NANOS_PER_UNIT;
  return {
    units: Math.trunc(amount.units) + Math.trunc(nanos / NANOS_PER_UNIT),
    nanos: nanos % NANOS_PER_UNIT,
  };
}

function isSupported(code) {
  // hasOwn, so that a prototype key such as "toString" is not a currency.
  return Object.hasOwn(ratesPerEuro, code);
}

// Through the euro, the base of the rates table.
function convertAmount(from, toCode) {
  const fromRate = Number(ratesPerEuro[from.currency_code]);
  const euros = carry({
    units: from.units / fromRate,
    nanos: from.nanos / fromRate,
  });
  euros.nanos = Math.round(euros.nanos);

  const toRate = Number(ratesPerEuro[toCode]);
  const result = carry({
    units: euros.units * toRate,
    nanos: euros.nanos * toRate,
  });
  return {
    units: result.units,
    nanos: Math.trunc(result.nanos),
    currency_code: toCode,
  };
}

function getSupportedCurrencies(call, callback) {
  logger.debug('Getting supported currencies...');
  callback(null, { currency_codes: Object.keys(ratesPerEuro) });
}

function convert(call, callback) {
  try {
    const { from, to_code: toCode } = call.request;
    // An unknown code would turn the amount into NaN, which is sent as 0.
    for (const code of [from?.currency_code ?? '', toCode ?? '']) {
      if (!isSupported(code)) {
        logger.warn(
          `conversion request rejected: unsupported currency code "${code}"`,
        );
        callback({
          code: grpc.status.INVALID_ARGUMENT,
          message: `unsupported currency code "${code}"`,
        });
        return;
      }
    }
    const result = convertAmount(from, toCode);
    logger.debug('conversion request successful');
    callback(null, result);
  } catch (err) {
    // A bug: the detail stays in the logs.
    logger.error(`conversion request failed: ${err.stack}`);
    callback({ code: grpc.status.INTERNAL, message: 'internal error' });
  }
}

// Set on SIGTERM so the health check stops traffic while in-flight calls finish.
let shuttingDown = false;

function check(call, callback) {
  callback(null, { status: shuttingDown ? 'NOT_SERVING' : 'SERVING' });
}

function createServer() {
  const server = new grpc.Server();
  server.addService(shopProto.CurrencyService.service, {
    getSupportedCurrencies,
    convert,
  });
  server.addService(healthProto.Health.service, { check });
  return server;
}

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

// Needed: as PID 1 in the container, Node would otherwise ignore SIGTERM.
function stopOnSignals(server) {
  for (const signal of ['SIGTERM', 'SIGINT']) {
    process.once(signal, async () => {
      logger.info(`received ${signal}, shutting down`);
      shuttingDown = true;
      await gracefulShutdown(server);
      process.exit(0);
    });
  }
}

// Exits if the port cannot be bound, instead of staying up without serving.
function start(port) {
  const server = createServer();
  server.bindAsync(
    `[::]:${port}`,
    grpc.ServerCredentials.createInsecure(),
    (err) => {
      if (err) {
        logger.fatal(`cannot listen on port ${port}: ${err.message}`);
        process.exit(1);
      }
      logger.info(`CurrencyService gRPC server started on port ${port}`);
    },
  );
  stopOnSignals(server);
  return server;
}

module.exports = {
  carry,
  check,
  convert,
  createServer,
  getSupportedCurrencies,
  gracefulShutdown,
  logger,
  start,
  stopOnSignals,
};

if (!!(require.main === module)) {
  start(process.env.PORT || '7000');
}
