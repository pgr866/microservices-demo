// Copyright 2024 Google LLC
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

// Package validator checks the forms the shop receives with the standard
// library only: a general-purpose validation library pulled in six more
// modules for these few rules.
package validator

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"
)

type AddToCartPayload struct {
	Quantity  uint64
	ProductID string
}

type PlaceOrderPayload struct {
	Email         string
	StreetAddress string
	ZipCode       int64
	City          string
	State         string
	Country       string
	CcNumber      string
	CcMonth       int64
	CcYear        int64
	CcCVV         int64
}

type SetCurrencyPayload struct {
	Currency string
	// Allowed are the currencies the shop accepts.
	Allowed map[string]bool
}

func (ad *AddToCartPayload) Validate() error {
	var c checker
	c.check("Quantity", "required", ad.Quantity != 0)
	c.check("Quantity", "gte", ad.Quantity >= 1)
	c.check("Quantity", "lte", ad.Quantity <= 10)
	c.check("ProductID", "required", ad.ProductID != "")
	return c.err()
}

func (po *PlaceOrderPayload) Validate() error {
	var c checker
	c.check("Email", "required", po.Email != "")
	c.check("Email", "email", isEmail(po.Email))
	c.requiredMaxLength("StreetAddress", po.StreetAddress, 512)
	c.check("ZipCode", "required", po.ZipCode != 0)
	c.requiredMaxLength("City", po.City, 128)
	c.requiredMaxLength("State", po.State, 128)
	c.requiredMaxLength("Country", po.Country, 128)
	c.check("CcNumber", "required", po.CcNumber != "")
	c.check("CcNumber", "credit_card", isCreditCard(po.CcNumber))
	c.check("CcMonth", "required", po.CcMonth != 0)
	c.check("CcMonth", "gte", po.CcMonth >= 1)
	c.check("CcMonth", "lte", po.CcMonth <= 12)
	c.check("CcYear", "required", po.CcYear != 0)
	c.check("CcCVV", "required", po.CcCVV != 0)
	return c.err()
}

func (sc *SetCurrencyPayload) Validate() error {
	var c checker
	c.check("Currency", "required", sc.Currency != "")
	c.check("Currency", "oneof", sc.Allowed[sc.Currency])
	return c.err()
}

type FieldError struct {
	Field string
	Rule  string
}

// Errors lists the fields that failed validation, each with the first rule it
// broke.
type Errors []FieldError

func (e Errors) Error() string {
	var msg strings.Builder
	for _, fe := range e {
		fmt.Fprintf(&msg, "Field '%s' is invalid: %s\n", fe.Field, fe.Rule)
	}
	return msg.String()
}

// checker collects the rules a payload breaks, keeping only the first one per
// field.
type checker struct {
	errs Errors
}

func (c *checker) check(field, rule string, ok bool) {
	if ok {
		return
	}
	for _, fe := range c.errs {
		if fe.Field == field {
			return
		}
	}
	c.errs = append(c.errs, FieldError{Field: field, Rule: rule})
}

func (c *checker) requiredMaxLength(field, value string, max int) {
	c.check(field, "required", value != "")
	c.check(field, "max", utf8.RuneCountInString(value) <= max)
}

func (c *checker) err() error {
	if len(c.errs) == 0 {
		return nil
	}
	return c.errs
}

// isEmail accepts a bare address (no display name) whose domain has a dot.
func isEmail(s string) bool {
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return false
	}
	domain := s[strings.LastIndex(s, "@")+1:]
	return strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

// isCreditCard accepts 12 to 19 digits, optionally in space-separated groups
// of at least 3, with a valid Luhn checksum.
func isCreditCard(s string) bool {
	var digits strings.Builder
	for group := range strings.SplitSeq(s, " ") {
		if len(group) < 3 {
			return false
		}
		digits.WriteString(group)
	}
	n := digits.Len()
	return n >= 12 && n <= 19 && luhn(digits.String())
}

// luhn reports whether digits (and only digits) pass the Luhn checksum that
// card numbers carry in their last digit.
func luhn(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i]) - '0'
		if d < 0 || d > 9 {
			return false
		}
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func ValidationErrorResponse(err error) error {
	var validationErrs Errors
	if !errors.As(err, &validationErrs) {
		return errors.New("invalid validation error format")
	}
	return errors.New(validationErrs.Error())
}
