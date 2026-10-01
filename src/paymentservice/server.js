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

const path = require('path');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');

const charge = require('./charge');

const logger = require('./logger');

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

class HipsterShopServer {
  constructor(protoRoot, port = HipsterShopServer.PORT) {
    this.port = port;

    this.packages = {
      hipsterShop: this.loadProto(path.join(protoRoot, 'demo.proto')),
      health: this.loadProto(
        path.join(protoRoot, 'grpc/health/v1/health.proto'),
      ),
    };

    this.server = new grpc.Server();
    this.loadAllProtos();
  }

  static ChargeServiceHandler(call, callback) {
    try {
      // Not the request itself: it carries the full card number and CVV, which
      // must never be stored, logs included (PCI DSS).
      logger.info('PaymentService#Charge invoked');
      const response = charge(call.request);
      callback(null, response);
    } catch (err) {
      logger.warn(`PaymentService#Charge failed: ${err.message}`);
      // Card errors carry INVALID_ARGUMENT; anything else is a bug.
      callback({
        code: err.code ?? grpc.status.INTERNAL,
        message: err.message,
      });
    }
  }

  static CheckHandler(call, callback) {
    callback(null, { status: 'SERVING' });
  }

  listen() {
    const server = this.server;
    const port = this.port;
    server.bindAsync(
      `[::]:${port}`,
      grpc.ServerCredentials.createInsecure(),
      function () {
        logger.info(`PaymentService gRPC server started on port ${port}`);
      },
    );
  }

  loadProto(path) {
    const packageDefinition = protoLoader.loadSync(path, {
      keepCase: true,
      longs: String,
      enums: String,
      defaults: true,
      oneofs: true,
    });
    return grpc.loadPackageDefinition(packageDefinition);
  }

  loadAllProtos() {
    const hipsterShopPackage = this.packages.hipsterShop.hipstershop;
    const healthPackage = this.packages.health.grpc.health.v1;

    this.server.addService(hipsterShopPackage.PaymentService.service, {
      charge: HipsterShopServer.ChargeServiceHandler.bind(this),
    });

    this.server.addService(healthPackage.Health.service, {
      check: HipsterShopServer.CheckHandler.bind(this),
    });
  }
}

HipsterShopServer.PORT = process.env.PORT;

module.exports = HipsterShopServer;
module.exports.gracefulShutdown = gracefulShutdown;
module.exports.stopOnSignals = stopOnSignals;
