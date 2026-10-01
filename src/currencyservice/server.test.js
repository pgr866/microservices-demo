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

const { status } = require('@grpc/grpc-js');
const { _carry, convert, getSupportedCurrencies, check } = require('./server');

describe('_carry', () => {
  it('leaves a whole amount untouched', () => {
    expect(_carry({ units: 10, nanos: 0 })).toEqual({ units: 10, nanos: 0 });
  });

  it('carries a fractional unit into nanos', () => {
    expect(_carry({ units: 10.5, nanos: 0 })).toEqual({
      units: 10,
      nanos: 500000000,
    });
  });

  it('carries nanos overflow into units', () => {
    expect(_carry({ units: 10, nanos: 1500000000 })).toEqual({
      units: 11,
      nanos: 500000000,
    });
  });
});

describe('convert', () => {
  it('converts between two currencies using the real conversion table', () => {
    const callback = jest.fn();
    convert(
      {
        request: {
          from: { currency_code: 'EUR', units: 10, nanos: 0 },
          to_code: 'USD',
        },
      },
      callback,
    );

    expect(callback).toHaveBeenCalledTimes(1);
    const [err, result] = callback.mock.calls[0];
    expect(err).toBeNull();
    expect(result.currency_code).toBe('USD');
    // 10 EUR at a rate of 1.1305 is 11.305 USD (units + nanos) — allow a tiny
    // floating-point tolerance instead of asserting an exact nanos value.
    expect(result.units).toBe(11);
    expect(result.nanos).toBeGreaterThan(304999000);
    expect(result.nanos).toBeLessThan(305001000);
  });

  it('round-trips EUR to EUR as a no-op', () => {
    const callback = jest.fn();
    convert(
      {
        request: {
          from: { currency_code: 'EUR', units: 42, nanos: 0 },
          to_code: 'EUR',
        },
      },
      callback,
    );

    expect(callback).toHaveBeenCalledWith(null, {
      units: 42,
      nanos: 0,
      currency_code: 'EUR',
    });
  });
});

describe('convert with invalid input', () => {
  it.each([
    [
      'an unsupported target currency',
      { from: { currency_code: 'EUR', units: 1, nanos: 0 }, to_code: 'XYZ' },
      'XYZ',
    ],
    [
      'an unsupported source currency',
      { from: { currency_code: 'XYZ', units: 1, nanos: 0 }, to_code: 'EUR' },
      'XYZ',
    ],
    ['a missing amount', { from: null, to_code: 'EUR' }, ''],
    [
      'a prototype key as currency',
      {
        from: { currency_code: 'EUR', units: 1, nanos: 0 },
        to_code: 'toString',
      },
      'toString',
    ],
  ])('rejects %s with INVALID_ARGUMENT', (_name, request, code) => {
    const callback = jest.fn();
    convert({ request }, callback);

    expect(callback).toHaveBeenCalledWith({
      code: status.INVALID_ARGUMENT,
      message: `unsupported currency code "${code}"`,
    });
  });
});

describe('getSupportedCurrencies', () => {
  it('lists the currency codes from the conversion table', () => {
    const callback = jest.fn();
    getSupportedCurrencies({}, callback);

    expect(callback).toHaveBeenCalledTimes(1);
    const [err, result] = callback.mock.calls[0];
    expect(err).toBeNull();
    expect(result.currency_codes).toEqual(
      expect.arrayContaining(['EUR', 'USD']),
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

describe('graceful shutdown', () => {
  const { gracefulShutdown, stopOnSignals } = require('./server');

  afterEach(() => {
    process.removeAllListeners('SIGTERM');
    process.removeAllListeners('SIGINT');
    jest.restoreAllMocks();
  });

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
});
