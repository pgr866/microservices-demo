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

const crypto = require('crypto');
const { status } = require('@grpc/grpc-js');
const cardValidator = require('simple-card-validator');
const pino = require('pino');

const logger = pino({
  name: 'paymentservice-charge',
  messageKey: 'message',
  formatters: {
    level(logLevelString) {
      return { severity: logLevelString };
    },
  },
});

class CreditCardError extends Error {
  constructor(message) {
    super(message);
    this.code = status.INVALID_ARGUMENT;
  }
}

class InvalidCreditCard extends CreditCardError {
  constructor() {
    super(`Credit card info is invalid`);
  }
}

class UnacceptedCreditCard extends CreditCardError {
  constructor(cardType) {
    super(
      `Sorry, we cannot process ${cardType} credit cards. Only VISA or MasterCard is accepted.`,
    );
  }
}

class ExpiredCreditCard extends CreditCardError {
  constructor(number, month, year) {
    super(
      `Your credit card (ending ${number.substr(-4)}) expired on ${month}\/${year}`,
    );
  }
}

class InvalidAmount extends Error {
  constructor() {
    super('The amount to charge must be positive and have a currency code');
    this.code = status.INVALID_ARGUMENT;
  }
}

// A zero or negative amount would be accepted as a charge (a negative one is
// in effect a refund).
function isPositiveAmount(amount) {
  if (!amount?.currency_code) {
    return false;
  }
  const units = Number(amount.units);
  const nanos = Number(amount.nanos);
  return units > 0 || (units === 0 && nanos > 0);
}

// Validates the card and only pretends to charge it: no payment is made.
module.exports = function charge(request) {
  const { amount, credit_card: creditCard } = request;
  if (!isPositiveAmount(amount)) {
    throw new InvalidAmount();
  }
  // Checked here because simple-card-validator throws a plain Error without one.
  if (!creditCard?.credit_card_number) {
    throw new InvalidCreditCard();
  }
  const cardNumber = creditCard.credit_card_number;
  const cardInfo = cardValidator(cardNumber);
  const { card_type: cardType, valid } = cardInfo.getCardDetails();

  if (!valid) {
    throw new InvalidCreditCard();
  }

  if (!(cardType === 'visa' || cardType === 'mastercard')) {
    throw new UnacceptedCreditCard(cardType);
  }

  const currentMonth = new Date().getMonth() + 1;
  const currentYear = new Date().getFullYear();
  const {
    credit_card_expiration_year: year,
    credit_card_expiration_month: month,
  } = creditCard;
  if (currentYear * 12 + currentMonth > year * 12 + month) {
    throw new ExpiredCreditCard(cardNumber.replace('-', ''), month, year);
  }

  logger.info(`Transaction processed: ${cardType} ending ${cardNumber.substr(-4)} \
    Amount: ${amount.currency_code}${amount.units}.${amount.nanos}`);

  return { transaction_id: crypto.randomUUID() };
};

module.exports.logger = logger;
