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

const charge = require('./charge');

const amount = { currency_code: 'USD', units: 100, nanos: 0 };
const futureYear = new Date().getFullYear() + 1;
const pastYear = new Date().getFullYear() - 1;

function buildRequest(cardNumber, year, month) {
  return {
    amount,
    credit_card: {
      credit_card_number: cardNumber,
      credit_card_expiration_year: year,
      credit_card_expiration_month: month,
    },
  };
}

describe('charge', () => {
  it('charges a valid Visa card and returns a transaction id', () => {
    const result = charge(buildRequest('4111111111111111', futureYear, 1));
    expect(result.transaction_id).toBeDefined();
  });

  it('charges a valid Mastercard card', () => {
    const result = charge(buildRequest('5555555555554444', futureYear, 1));
    expect(result.transaction_id).toBeDefined();
  });

  it('rejects an invalid card number', () => {
    expect(() => charge(buildRequest('4111111111111112', futureYear, 1)))
      .toThrow('Credit card info is invalid');
  });

  it('rejects card types other than Visa/Mastercard', () => {
    expect(() => charge(buildRequest('378282246310005', futureYear, 1)))
      .toThrow(/cannot process/);
  });

  it('rejects an expired card', () => {
    expect(() => charge(buildRequest('4111111111111111', pastYear, 1)))
      .toThrow(/expired/);
  });
});
