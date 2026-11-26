// 50 users add the same product to the same cart at the same time: the cart
// must end up with all 50 (cartservice's atomic HINCRBY on Redis).
import http from 'k6/http';
import { check } from 'k6';
import {
  BASE_URL,
  SUNGLASSES,
  addToCart,
  cart,
  home,
  sessionCookie,
} from './lib/shop.js';

const USERS = 50;

export const options = {
  scenarios: {
    together: { executor: 'per-vu-iterations', vus: USERS, iterations: 1 },
  },
  thresholds: { checks: ['rate==1'], http_req_failed: ['rate==0'] },
};

// Each VU has its own cookie jar: they all get the same session cookie.
const useSession = (id) =>
  http.cookieJar().set(BASE_URL, 'shop_session-id', id);

export function setup() {
  home();
  const session = sessionCookie();
  if (session === undefined) {
    throw new Error('frontend did not set a session cookie');
  }
  return session;
}

export default function (session) {
  useSession(session);
  check(addToCart(SUNGLASSES, 1), {
    'every addition is 200': (r) => r.status === 200,
  });
}

export function teardown(session) {
  useSession(session);
  check(cart(), {
    [`the cart has all ${USERS} units`]: (r) =>
      r.body.includes(`Quantity: ${USERS}`),
  });
}
