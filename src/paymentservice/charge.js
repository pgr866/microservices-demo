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

const crypto = require('node:crypto');
const { status } = require('@grpc/grpc-js');
const cardValidator = require('simple-card-validator');
const logger = require('./logger')('paymentservice-charge');

class InvalidArgumentError extends Error {
  constructor(message) {
    super(message);
    this.code = status.INVALID_ARGUMENT;
  }
}

class InvalidCreditCard extends InvalidArgumentError {
  constructor() {
    super('Credit card info is invalid');
  }
}

class UnacceptedCreditCard extends InvalidArgumentError {
  constructor(cardType) {
    super(
      `Sorry, we cannot process ${cardType} credit cards. Only VISA or MasterCard is accepted.`,
    );
  }
}

class ExpiredCreditCard extends InvalidArgumentError {
  constructor(number, month, year) {
    super(
      `Your credit card (ending ${number.slice(-4)}) expired on ${month}/${year}`,
    );
  }
}

class InvalidAmount extends InvalidArgumentError {
  constructor() {
    super('The amount to charge must be positive and have a currency code');
  }
}

// A negative amount would in effect be a refund; google.type.Money also
// forbids units and nanos with different signs.
function isPositiveAmount(amount) {
  if (!amount?.currency_code) {
    return false;
  }
  const units = Number(amount.units);
  const nanos = Number(amount.nanos);
  return (units > 0 && nanos >= 0) || (units === 0 && nanos > 0);
}

// In cents, zero-padded: 0.05 is 50000000 nanos.
function formatAmount({ currency_code: currency, units, nanos }) {
  const cents = String(Math.floor(nanos / 10000000)).padStart(2, '0');
  return `${currency} ${units}.${cents}`;
}

// Validates the card and only pretends to charge it: no payment is made.
module.exports = function charge(request) {
  const { amount, credit_card: creditCard } = request;
  if (!isPositiveAmount(amount)) {
    throw new InvalidAmount();
  }
  // simple-card-validator throws a plain Error without a number.
  if (!creditCard?.credit_card_number) {
    throw new InvalidCreditCard();
  }
  const cardNumber = creditCard.credit_card_number;
  const { card_type: cardType, valid } =
    cardValidator(cardNumber).getCardDetails();

  if (!valid) {
    throw new InvalidCreditCard();
  }

  if (cardType !== 'visa' && cardType !== 'mastercard') {
    throw new UnacceptedCreditCard(cardType);
  }

  const {
    credit_card_expiration_year: year,
    credit_card_expiration_month: month,
  } = creditCard;
  if (!Number.isInteger(month) || month < 1 || month > 12) {
    throw new InvalidCreditCard();
  }

  const now = new Date();
  const currentMonth = now.getMonth() + 1;
  const currentYear = now.getFullYear();
  if (!!(currentYear * 12 + currentMonth > year * 12 + month)) {
    throw new ExpiredCreditCard(cardNumber, month, year);
  }

  logger.info(
    `Transaction processed: ${cardType} ending ${cardNumber.slice(-4)}, amount ${formatAmount(amount)}`,
  );

  return { transaction_id: crypto.randomUUID() };
};

module.exports.logger = logger;
