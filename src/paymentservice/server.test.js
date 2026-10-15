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

const grpc = require('@grpc/grpc-js');
const {
  chargeHandler,
  check,
  createServer,
  gracefulShutdown,
  logger: serverLogger,
  start,
  stopOnSignals,
} = require('./server');
const chargeLogger = require('./charge').logger;

const futureYear = new Date().getFullYear() + 1;

function chargeRequest(cardNumber, year = futureYear) {
  return {
    amount: { currency_code: 'USD', units: '100', nanos: 0 },
    credit_card: {
      credit_card_number: cardNumber,
      credit_card_cvv: 672,
      credit_card_expiration_year: year,
      credit_card_expiration_month: 1,
    },
  };
}

afterEach(() => {
  process.removeAllListeners('SIGTERM');
  process.removeAllListeners('SIGINT');
  jest.restoreAllMocks();
});

describe('createServer', () => {
  it('loads the real protos and builds the server without listening', () => {
    expect(createServer()).toBeInstanceOf(grpc.Server);
  });
});

describe('check', () => {
  it('reports SERVING', () => {
    const callback = jest.fn();
    check({}, callback);
    expect(callback).toHaveBeenCalledWith(null, { status: 'SERVING' });
  });
});

describe('chargeHandler', () => {
  it('responds with the transaction id of a valid charge', () => {
    const callback = jest.fn();
    chargeHandler({ request: chargeRequest('4111111111111111') }, callback);

    expect(callback).toHaveBeenCalledWith(null, {
      transaction_id: expect.stringMatching(/^[0-9a-f-]{36}$/),
    });
  });

  it.each([
    [
      'an invalid card',
      chargeRequest('4111111111111112'),
      'Credit card info is invalid',
    ],
    [
      'an expired card',
      chargeRequest('4111111111111111', 2000),
      'Your credit card (ending 1111) expired on 1/2000',
    ],
    [
      'a zero amount',
      {
        ...chargeRequest('4111111111111111'),
        amount: { currency_code: 'USD', units: '0', nanos: 0 },
      },
      'The amount to charge must be positive and have a currency code',
    ],
  ])(
    'responds INVALID_ARGUMENT with the reason to %s',
    (_name, request, message) => {
      const callback = jest.fn();
      chargeHandler({ request }, callback);

      expect(callback).toHaveBeenCalledWith({
        code: grpc.status.INVALID_ARGUMENT,
        message,
      });
    },
  );

  it('responds INTERNAL to an unexpected error instead of throwing', () => {
    const callback = jest.fn();
    // Without a request, charge() fails with a TypeError.
    chargeHandler({}, callback);

    expect(callback).toHaveBeenCalledWith({
      code: grpc.status.INTERNAL,
      message: 'internal error',
    });
  });

  it.each([
    ['a successful charge', futureYear],
    ['a declined card', 2000],
  ])('never logs the card number or the CVV for %s', (_name, year) => {
    const lines = [];
    for (const l of [serverLogger, chargeLogger]) {
      for (const level of ['debug', 'info', 'warn', 'error']) {
        jest.spyOn(l, level).mockImplementation((msg) => lines.push(msg));
      }
    }
    chargeHandler(
      { request: chargeRequest('4432801561520454', year) },
      jest.fn(),
    );

    expect(lines.length).toBeGreaterThan(0);
    for (const line of lines) {
      expect(line).not.toContain('4432801561520454');
      expect(line).not.toContain('672');
    }
  });
});

describe('start', () => {
  it('exits with an error if the port cannot be bound', async () => {
    const exited = new Promise((resolve) => {
      jest.spyOn(process, 'exit').mockImplementation(resolve);
    });

    const server = start('not-a-port');

    await expect(exited).resolves.toBe(1);
    server.forceShutdown();
  });
});

describe('graceful shutdown', () => {
  it('lets in-flight calls finish without cutting them', async () => {
    const server = { tryShutdown: (done) => done(), forceShutdown: jest.fn() };

    await gracefulShutdown(server, 1000);

    expect(server.forceShutdown).not.toHaveBeenCalled();
  });

  it('cuts the calls still in flight after the timeout', async () => {
    const server = { tryShutdown: () => {}, forceShutdown: jest.fn() };

    await gracefulShutdown(server, 10);

    expect(server.forceShutdown).toHaveBeenCalledTimes(1);
  });

  it.each(['SIGTERM', 'SIGINT'])(
    'shuts down and exits on %s',
    async (signal) => {
      const exit = jest.spyOn(process, 'exit').mockImplementation(() => {});
      const server = {
        tryShutdown: jest.fn((done) => done()),
        forceShutdown: jest.fn(),
      };
      stopOnSignals(server);

      process.emit(signal);
      await new Promise(setImmediate);

      expect(server.tryShutdown).toHaveBeenCalledTimes(1);
      expect(exit).toHaveBeenCalledWith(0);
    },
  );

  it('reports NOT_SERVING once shutdown starts, before draining', () => {
    jest.isolateModules(() => {
      const fresh = require('./server');
      const server = { tryShutdown: jest.fn(), forceShutdown: jest.fn() };
      const callback = jest.fn();
      fresh.stopOnSignals(server);

      fresh.check({}, callback);
      expect(callback).toHaveBeenLastCalledWith(null, { status: 'SERVING' });

      jest.useFakeTimers();
      process.emit('SIGTERM');
      fresh.check({}, callback);

      expect(callback).toHaveBeenLastCalledWith(null, {
        status: 'NOT_SERVING',
      });
      expect(server.tryShutdown).toHaveBeenCalledTimes(1);
      jest.clearAllTimers();
      jest.useRealTimers();
    });
  });
});
