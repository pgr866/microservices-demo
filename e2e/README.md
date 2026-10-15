# End-to-end tests

The whole shop with the production images of the 10 services, each run like in Kubernetes (read-only filesystem, no capabilities, no privilege escalation), and [k6](https://k6.io) going through `frontend` as a user would.

- `smoke.js`: every page and form. It checks a full purchase, the cart, the currency change, and errors that must reach the browser with the right status: unknown product 404, invalid form 422, expired card 400, empty cart 400, and a cross-site POST 403. It also checks the security headers and that the session ID never shows up in the page.
- `cart-concurrent.js`: 50 users add the same product to the same cart at once, and the cart must end up with all 50 (atomic increments in Redis).
- `payment-hung.js`: run with `paymentservice` paused, so that it takes the call but never answers. The order must fail with 504 when the 5 s request deadline runs out, show no internal details, and keep the cart. When `run.sh` resumes it, `paymentservice` still processes that pending charge, so with `KEEP=1` its log shows a transaction with no order.
- `redis-down.js`: run with `redis-cart` stopped. The cart page must fail at once with 503, not wait out the deadline, and show no internal details.
- `lib/shop.js`: the shop's pages and forms, shared with the load test journeys.

Before k6 starts, the `ready` service waits, for up to 120 s, until every service answers its health check (gRPC `Health/Check` with `SERVING`, `/_healthz` for frontend, and the port of `redis-cart`), like the Kubernetes readiness probes. If a service doesn't answer, the run fails and names it.

## Run

```sh
./run.sh
```

`run.sh` builds the images, starts the shop and runs the four tests (pausing `paymentservice` and stopping `redis-cart` for the last two). It removes the containers at the end; set `KEEP=1` to keep the shop running. A failed check makes k6, and so `run.sh`, exit non-zero.

With the shop running (`KEEP=1`), a single test:

```sh
docker compose run --rm k6 run smoke.js
```

## Run in Kubernetes

`kustomization.yaml` runs `smoke.js` and then `cart-concurrent.js`, the tests that need no fault injected, as a Job (`job.yaml`) in the namespace of the shop under test and against its `frontend` Service, with the same scripts from a ConfigMap. Its NetworkPolicy lets k6 reach `frontend` (and DNS) also under the `deny-all` policy of the hardened scenario. From the repo root:

```sh
kubectl apply -k e2e -n <namespace>
kubectl logs -f job/e2e -n <namespace> --pod-running-timeout=2m
kubectl wait --for=condition=complete --timeout=10s job/e2e -n <namespace>
kubectl delete -k e2e -n <namespace>
```

`kubectl wait` fails unless every check passed. The Job is not retried (`backoffLimit: 0`); to run it again, delete it first.
