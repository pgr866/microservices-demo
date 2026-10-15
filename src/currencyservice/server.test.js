// Copyright 2018 Google LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const grpc = require('@grpc/grpc-js');
const {
  carry,
  check,
  convert,
  createServer,
  getSupportedCurrencies,
  gracefulShutdown,
  start,
  stopOnSignals,
} = require('./server');
const rates = require('./data/currency_conversion.json');

function convertRequest(request) {
  const callback = jest.fn();
  convert({ request }, callback);
  expect(callback).toHaveBeenCalledTimes(1);
  return callback.mock.calls[0];
}

function inNanos({ units, nanos }) {
  return Number(units) * 1e9 + nanos;
}

afterEach(() => {
  process.removeAllListeners('SIGTERM');
  process.removeAllListeners('SIGINT');
  jest.restoreAllMocks();
});

describe('carry', () => {
  it.each([
    [
      'leaves a whole amount untouched',
      { units: 10, nanos: 0 },
      { units: 10, nanos: 0 },
    ],
    [
      'moves a fractional unit into nanos',
      { units: 10.5, nanos: 0 },
      { units: 10, nanos: 500000000 },
    ],
    [
      'moves whole units out of nanos',
      { units: 10, nanos: 1500000000 },
      { units: 11, nanos: 500000000 },
    ],
    // Regression: flooring turned -1.5 into -3 units and -0.5 nanos.
    [
      'keeps units and nanos negative',
      { units: -1.5, nanos: 0 },
      { units: -1, nanos: -500000000 },
    ],
  ])('%s', (_name, amount, expected) => {
    expect(carry(amount)).toEqual(expected);
  });
});

describe('convert', () => {
  it('converts from the euro with the rate of the table', () => {
    const [err, result] = convertRequest({
      from: { currency_code: 'EUR', units: '10', nanos: 0 },
      to_code: 'USD',
    });

    expect(err).toBeNull();
    expect(result.currency_code).toBe('USD');
    // 10 EUR at 1.1305 is 11.305 USD, give or take floating-point error.
    expect(result.units).toBe(11);
    expect(Math.abs(result.nanos - 305000000)).toBeLessThan(1000);
  });

  it('converts between two other currencies through the euro, nanos included', () => {
    const [err, result] = convertRequest({
      from: { currency_code: 'USD', units: '11', nanos: 305000000 },
      to_code: 'JPY',
    });

    expect(err).toBeNull();
    expect(result.currency_code).toBe('JPY');
    // 11.305 USD at 1.1305 is 10 EUR, which at 126.40 is 1264 JPY.
    expect(Math.abs(inNanos(result) - 1264e9)).toBeLessThan(1e6);
  });

  it('returns the same amount when converting to the same currency', () => {
    const [, result] = convertRequest({
      from: { currency_code: 'EUR', units: '42', nanos: 250000000 },
      to_code: 'EUR',
    });

    expect(result).toEqual({
      units: 42,
      nanos: 250000000,
      currency_code: 'EUR',
    });
  });

  it('keeps the sign of a negative amount in units and nanos', () => {
    const [, result] = convertRequest({
      from: { currency_code: 'EUR', units: '-1', nanos: -500000000 },
      to_code: 'EUR',
    });

    expect(result).toEqual({
      units: -1,
      nanos: -500000000,
      currency_code: 'EUR',
    });
  });

  it('returns integer units and nanos, as the proto requires', () => {
    const [, result] = convertRequest({
      from: { currency_code: 'USD', units: '7', nanos: 990000000 },
      to_code: 'GBP',
    });

    expect(Number.isInteger(result.units)).toBe(true);
    expect(Number.isInteger(result.nanos)).toBe(true);
    expect(result.nanos).toBeGreaterThanOrEqual(0);
    expect(result.nanos).toBeLessThan(1e9);
  });

  it.each([
    [
      'an unsupported target currency',
      { from: { currency_code: 'EUR', units: '1', nanos: 0 }, to_code: 'XYZ' },
      'XYZ',
    ],
    [
      'an unsupported source currency',
      { from: { currency_code: 'XYZ', units: '1', nanos: 0 }, to_code: 'EUR' },
      'XYZ',
    ],
    [
      'a lowercase code',
      { from: { currency_code: 'eur', units: '1', nanos: 0 }, to_code: 'USD' },
      'eur',
    ],
    ['a missing amount', { from: null, to_code: 'EUR' }, ''],
    [
      'a missing target currency',
      { from: { currency_code: 'EUR', units: '1', nanos: 0 } },
      '',
    ],
    [
      'a prototype key as currency',
      {
        from: { currency_code: 'EUR', units: '1', nanos: 0 },
        to_code: 'toString',
      },
      'toString',
    ],
  ])('rejects %s with INVALID_ARGUMENT', (_name, request, code) => {
    expect(convertRequest(request)).toEqual([
      {
        code: grpc.status.INVALID_ARGUMENT,
        message: `unsupported currency code "${code}"`,
      },
    ]);
  });

  it('responds INTERNAL to an unexpected error instead of throwing', () => {
    const callback = jest.fn();
    // Without a request, reading its fields throws a TypeError.
    convert({}, callback);

    expect(callback).toHaveBeenCalledWith({
      code: grpc.status.INTERNAL,
      message: 'internal error',
    });
  });
});

describe('getSupportedCurrencies', () => {
  it('lists every currency of the rates table', () => {
    const callback = jest.fn();
    getSupportedCurrencies({}, callback);

    expect(callback).toHaveBeenCalledWith(null, {
      currency_codes: Object.keys(rates),
    });
    expect(Object.keys(rates)).toEqual(
      expect.arrayContaining(['EUR', 'USD', 'JPY']),
    );
  });
});

describe('check', () => {
  it('reports SERVING', () => {
    const callback = jest.fn();
    check({}, callback);
    expect(callback).toHaveBeenCalledWith(null, { status: 'SERVING' });
  });
});

describe('createServer', () => {
  it('loads the real protos and builds the server without listening', () => {
    expect(createServer()).toBeInstanceOf(grpc.Server);
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
