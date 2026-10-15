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

const { status } = require('@grpc/grpc-js');
const charge = require('./charge');

const VISA = '4111111111111111';
const now = new Date();
const thisMonth = now.getMonth() + 1;
const thisYear = now.getFullYear();

function buildRequest({
  number = VISA,
  year = thisYear + 1,
  month = 1,
  amount = { currency_code: 'USD', units: '100', nanos: 0 },
} = {}) {
  return {
    amount,
    credit_card: {
      credit_card_number: number,
      credit_card_expiration_year: year,
      credit_card_expiration_month: month,
    },
  };
}

function invalidArgument(message) {
  return expect.objectContaining({ code: status.INVALID_ARGUMENT, message });
}

describe('charge', () => {
  it.each([
    ['Visa', VISA],
    ['Mastercard', '5555555555554444'],
  ])('charges a valid %s card and returns a random UUID', (_name, number) => {
    const first = charge(buildRequest({ number }));
    const second = charge(buildRequest({ number }));

    expect(first.transaction_id).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
    );
    expect(second.transaction_id).not.toBe(first.transaction_id);
  });

  it('accepts a card that expires this month', () => {
    const request = buildRequest({ year: thisYear, month: thisMonth });
    expect(charge(request).transaction_id).toBeDefined();
  });

  it('charges an amount of only cents', () => {
    const amount = { currency_code: 'USD', units: '0', nanos: 500000000 };
    expect(charge(buildRequest({ amount })).transaction_id).toBeDefined();
  });

  const lastMonth = thisMonth === 1 ? 12 : thisMonth - 1;
  const lastMonthYear = thisMonth === 1 ? thisYear - 1 : thisYear;

  it.each([
    [
      'an invalid card number',
      buildRequest({ number: '4111111111111112' }),
      'Credit card info is invalid',
    ],
    [
      'a request without a card',
      { amount: buildRequest().amount },
      'Credit card info is invalid',
    ],
    [
      'a card type other than Visa or Mastercard',
      buildRequest({ number: '378282246310005' }),
      'Sorry, we cannot process amex credit cards. Only VISA or MasterCard is accepted.',
    ],
    [
      'an expiration month of 0',
      buildRequest({ month: 0 }),
      'Credit card info is invalid',
    ],
    [
      'an expiration month of 13',
      buildRequest({ month: 13 }),
      'Credit card info is invalid',
    ],
    [
      'a card that expired last month',
      buildRequest({ year: lastMonthYear, month: lastMonth }),
      `Your credit card (ending 1111) expired on ${lastMonth}/${lastMonthYear}`,
    ],
  ])('rejects %s with INVALID_ARGUMENT', (_name, request, message) => {
    expect(() => charge(request)).toThrow(invalidArgument(message));
  });

  it.each([
    ['no amount', undefined],
    ['zero', { currency_code: 'USD', units: '0', nanos: 0 }],
    ['negative', { currency_code: 'USD', units: '-100', nanos: 0 }],
    [
      'negative cents only',
      { currency_code: 'USD', units: '0', nanos: -500000000 },
    ],
    [
      'units and nanos with different signs',
      { currency_code: 'USD', units: '1', nanos: -500000000 },
    ],
    ['no currency code', { currency_code: '', units: '100', nanos: 0 }],
  ])(
    'rejects an invalid amount (%s) before looking at the card',
    (_name, amount) => {
      // The card is invalid too: the amount is checked first.
      const request = {
        ...buildRequest({ number: '4111111111111112' }),
        amount,
      };
      expect(() => charge(request)).toThrow(
        invalidArgument(
          'The amount to charge must be positive and have a currency code',
        ),
      );
    },
  );

  it('logs the card type, last four digits and amount in zero-padded cents', () => {
    const info = jest.spyOn(charge.logger, 'info').mockImplementation(() => {});
    try {
      const amount = { currency_code: 'USD', units: '3', nanos: 50000000 };
      charge(buildRequest({ amount }));

      expect(info).toHaveBeenCalledWith(
        'Transaction processed: visa ending 1111, amount USD 3.05',
      );
    } finally {
      info.mockRestore();
    }
  });
});
