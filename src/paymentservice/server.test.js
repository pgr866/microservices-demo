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
const HipsterShopServer = require('./server');

const PROTO_PATH = path.join(__dirname, 'proto');

describe('HipsterShopServer', () => {
  it('loads the protos and builds a gRPC server without starting a network listener', () => {
    const server = new HipsterShopServer(PROTO_PATH, 50052);
    expect(server.server).toBeInstanceOf(grpc.Server);
    expect(server.packages.hipsterShop.hipstershop.PaymentService).toBeDefined();
    expect(server.packages.health.grpc.health.v1.Health).toBeDefined();
  });

  describe('CheckHandler', () => {
    it('reports SERVING', () => {
      const callback = jest.fn();
      HipsterShopServer.CheckHandler({}, callback);
      expect(callback).toHaveBeenCalledWith(null, { status: 'SERVING' });
    });
  });

  describe('ChargeServiceHandler', () => {
    const futureYear = new Date().getFullYear() + 1;

    it('charges a valid card and responds with a transaction id', () => {
      const call = {
        request: {
          amount: { currency_code: 'USD', units: 100, nanos: 0 },
          credit_card: {
            credit_card_number: '4111111111111111',
            credit_card_expiration_year: futureYear,
            credit_card_expiration_month: 1,
          },
        },
      };
      const callback = jest.fn();

      HipsterShopServer.ChargeServiceHandler(call, callback);

      expect(callback).toHaveBeenCalledTimes(1);
      const [err, response] = callback.mock.calls[0];
      expect(err).toBeNull();
      expect(response.transaction_id).toBeDefined();
    });

    it('passes the error to the callback for an invalid card', () => {
      const call = {
        request: {
          amount: { currency_code: 'USD', units: 100, nanos: 0 },
          credit_card: {
            credit_card_number: '4111111111111112',
            credit_card_expiration_year: futureYear,
            credit_card_expiration_month: 1,
          },
        },
      };
      const callback = jest.fn();

      HipsterShopServer.ChargeServiceHandler(call, callback);

      expect(callback).toHaveBeenCalledTimes(1);
      const [err] = callback.mock.calls[0];
      expect(err.message).toBe('Credit card info is invalid');
    });
  });
});
