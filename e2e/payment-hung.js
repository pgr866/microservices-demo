// Run with paymentservice paused (see run.sh): it accepts the call but never
// answers. The order must fail with 504 when frontend's request deadline
// (REQUEST_TIMEOUT, 5 s) runs out, not hang, without internal details reaching
// the browser, and keeping the cart.
import { check } from 'k6';
import { SUNGLASSES, addToCart, cart, checkout } from './lib/shop.js';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: { checks: ['rate==1'] },
};

export default function () {
  addToCart(SUNGLASSES, 1);

  check(checkout(), {
    'order fails with 504': (r) => r.status === 504,
    'when the 5 s deadline runs out': (r) =>
      r.timings.duration >= 5000 && r.timings.duration < 6000,
    'no internal details in the page': (r) =>
      !/rpc error|dial tcp|\.go:\d+/.test(r.body) &&
      r.body.includes('Request ID'),
  });
  check(cart(), {
    'the cart is kept for a later try': (r) => r.body.includes('Sunglasses'),
  });
}
