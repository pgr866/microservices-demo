// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const path = require('node:path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const charge = require('./charge');
const logger = require('./logger')('paymentservice-server');

// Below the 30 s Kubernetes waits before sending SIGKILL.
const SHUTDOWN_TIMEOUT_MS = 10000;

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

function chargeHandler(call, callback) {
  // Never the request: it carries the card number and CVV (PCI DSS).
  logger.debug('PaymentService#Charge invoked');
  try {
    callback(null, charge(call.request));
  } catch (err) {
    // Without a code it is a bug: its detail stays in the logs.
    if (err.code === undefined) {
      logger.error(`PaymentService#Charge failed: ${err.stack}`);
      callback({ code: grpc.status.INTERNAL, message: 'internal error' });
      return;
    }
    logger.warn(`PaymentService#Charge failed: ${err.message}`);
    callback({ code: err.code, message: err.message });
  }
}

// Set on SIGTERM so the health check stops traffic while in-flight calls finish.
let shuttingDown = false;

function check(call, callback) {
  callback(null, { status: shuttingDown ? 'NOT_SERVING' : 'SERVING' });
}

function createServer() {
  const server = new grpc.Server();
  server.addService(shopProto.PaymentService.service, {
    charge: chargeHandler,
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
      logger.info(`PaymentService gRPC server started on port ${port}`);
    },
  );
  stopOnSignals(server);
  return server;
}

module.exports = {
  chargeHandler,
  check,
  createServer,
  gracefulShutdown,
  logger,
  start,
  stopOnSignals,
};

if (require.main === module) {
  start(process.env.PORT || '50052');
}
