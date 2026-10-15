// End-to-end test of the whole shop through frontend, with every service
// real: one user goes through every page and form, including the failures
// that must reach the browser with the right status. Any failed check fails
// the run.
import http from 'k6/http';
import { check, group } from 'k6';
import {
  ADDRESS,
  SUNGLASSES,
  VALID_CARD,
  addToCart,
  cart,
  checkout,
  emptyCart,
  home,
  product,
  sessionCookie,
  setCurrency,
} from './lib/shop.js';

export const options = {
  vus: 1,
  iterations: 1,
  // Every check passes, and no request gets an unexpected status: also those
  // whose response is not checked (warm-up, setting up the cart).
  thresholds: { checks: ['rate==1'], http_req_failed: ['rate==0'] },
};

// 4xx are part of the test (expected errors); a 5xx is a failure.
http.setResponseCallback(http.expectedStatuses({ min: 200, max: 499 }));

// The services are ready (see compose.yaml): this only warms up the JVM of
// adservice, so that the ads fit in frontend's 100 ms ad deadline.
export function setup() {
  for (let i = 0; i < 5; i++) {
    product(SUNGLASSES);
  }
}

const productIds = (body) =>
  new Set(body.match(/\/product\/[A-Z0-9]{10}/g) ?? []).size;

export default function () {
  group('home', () => {
    const res = home();
    check(res, {
      'home is 200': (r) => r.status === 200,
      'home lists the 9 products': (r) => productIds(r.body) === 9,
      'home shows prices in USD by default': (r) => /\$\d+\.\d{2}/.test(r.body),
      'home sets the security headers': (r) =>
        r.headers['Content-Security-Policy']?.includes("default-src 'self'") &&
        r.headers['X-Frame-Options'] === 'DENY' &&
        r.headers['X-Content-Type-Options'] === 'nosniff',
      'session cookie is HttpOnly and SameSite=Lax': (r) => {
        const attrs = (r.headers['Set-Cookie'] ?? '')
          .split('\n')
          .find((c) => c.startsWith('shop_session-id='))
          ?.split(';')
          .map((a) => a.trim().toLowerCase());
        return attrs?.includes('httponly') && attrs?.includes('samesite=lax');
      },
      'home does not show the session ID': (r) => {
        const id = sessionCookie();
        return id !== undefined && !r.body.includes(id);
      },
    });
  });

  group('product', () => {
    check(product(SUNGLASSES), {
      'product is 200': (r) => r.status === 200,
      'product shows its name': (r) => r.body.includes('Sunglasses'),
      'product shows recommendations': (r) =>
        r.body.includes('You May Also Like'),
    });
    // Ads are optional by design (100 ms deadline, page served without them),
    // so one missing ad is not a failure; none in 5 tries is.
    const withAd = Array.from({ length: 5 }, () => product(SUNGLASSES)).some(
      (r) => r.body.includes('Watch for sale'),
    );
    check(withAd, { 'product shows an ad for its category': (found) => found });
    check(product('NOPE'), {
      'unknown product is 404': (r) => r.status === 404,
    });
  });

  group('cart', () => {
    check(addToCart(SUNGLASSES, 0), {
      'quantity 0 is 422': (r) => r.status === 422,
    });
    check(
      addToCart(SUNGLASSES, 1, { headers: { 'Sec-Fetch-Site': 'cross-site' } }),
      {
        'a cross-site POST is 403 (CSRF)': (r) => r.status === 403,
      },
    );
    check(cart(), {
      'rejected additions leave the cart empty': (r) =>
        r.body.includes('Your shopping cart is empty!'),
    });

    addToCart(SUNGLASSES, 1);
    check(addToCart(SUNGLASSES, 1), {
      'adding redirects to the cart, 200': (r) => r.status === 200,
    });
    check(cart(), {
      'cart adds up the same product': (r) =>
        r.body.includes('Sunglasses') && r.body.includes('Quantity: 2'),
    });

    emptyCart();
    check(cart(), {
      'emptying the cart works': (r) =>
        r.body.includes('Your shopping cart is empty!'),
    });
  });

  group('currency', () => {
    check(setCurrency('XYZ'), {
      'unknown currency is 422': (r) => r.status === 422,
    });
    check(setCurrency('EUR'), {
      'changing currency is 200': (r) => r.status === 200,
    });
    check(home(), {
      'home shows prices in EUR': (r) => /€\d+\.\d{2}/.test(r.body),
    });
  });

  group('checkout', () => {
    addToCart(SUNGLASSES, 2);

    check(checkout(VALID_CARD, { ...ADDRESS, email: 'not-an-email' }), {
      'an invalid form is 422': (r) => r.status === 422,
    });
    check(checkout({ ...VALID_CARD, credit_card_expiration_year: '2020' }), {
      'an expired card is 400 with the reason': (r) =>
        r.status === 400 && r.body.includes('expired on 1/2020'),
    });

    const order = checkout();
    check(order, {
      'order is 200': (r) => r.status === 200,
      'order shows the total paid in EUR': (r) =>
        /Total Paid[\s\S]*?€\d+\.\d{2}/.test(r.body),
      'order shows a tracking number': (r) =>
        /Tracking #[\s\S]*?[A-Z]{2}-\d+-\d+/.test(r.body),
    });
    check(cart(), {
      'the order empties the cart': (r) =>
        r.body.includes('Your shopping cart is empty!'),
    });
    check(checkout(), {
      'ordering with an empty cart is 400': (r) =>
        r.status === 400 && r.body.includes('cart is empty'),
    });
  });
}
