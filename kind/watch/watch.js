// A single user who loads a product page ten times a second and prints a line each time the
// answer changes: when the shop went down, and for how long it was up or down before.
import http from 'k6/http';
import { sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://frontend';
// Needs productcatalogservice, currencyservice and cartservice; adservice and
// recommendationservice are optional for this page.
const PAGE = `${BASE_URL}/product/OLJCESPC7Z`;

export const options = {
  scenarios: {
    watch: { executor: 'constant-vus', vus: 1, duration: '24h' },
  },
  discardResponseBodies: true,
};

// Kept across iterations: there is only one VU.
let current = null;

// HH:MM:SS.mmm in UTC, the time zone of the logs of the services.
const clock = (ms) => new Date(ms).toISOString().slice(11, 23);

export default function () {
  // A state starts when the first request that saw it was sent.
  const sent = Date.now();
  const res = http.get(PAGE, { timeout: '10s' });
  // 0: no HTTP answer at all (connection refused, reset or timed out).
  const status = res.status === 0 ? `0 (${res.error})` : String(res.status);

  if (current === null) {
    console.log(`${clock(sent)}  ${status}`);
  } else if (status !== current.status) {
    const seconds = ((sent - current.since) / 1000).toFixed(1);
    console.log(
      `${clock(sent)}  ${status}  after ${seconds} s of ${current.status} (${current.count} requests)`,
    );
  }
  if (current === null || status !== current.status) {
    current = { status, since: sent, count: 0 };
  }
  current.count++;
  sleep(0.1);
}
