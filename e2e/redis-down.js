// Run with redis-cart stopped (see run.sh): the cart's storage is down, so the
// cart page must fail at once with 503 (a dependency is unavailable, try again
// later): not a 500, not a 504 after waiting out the request deadline, and
// without internal details.
import { check } from 'k6';
import { cart } from './lib/shop.js';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: { checks: ['rate==1'] },
};

export default function () {
  check(cart(), {
    'the cart fails with 503': (r) => r.status === 503,
    'at once, without waiting for the deadline': (r) =>
      r.timings.duration < 1000,
    'no internal details in the page': (r) =>
      !/rpc error|dial tcp|\.go:\d+|Redis/.test(r.body) &&
      r.body.includes('Request ID'),
  });
}
