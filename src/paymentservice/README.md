# paymentservice

The Payment service charges the given credit card info (mock) with the given amount and returns a transaction ID.

## Development (hot reload, run from the repo root)

```bash
docker compose up paymentservice
```

## Build and run in production

```bash
docker build -t paymentservice:prod .
docker run --rm -p 50052:50052 -e PORT=50052 paymentservice:prod
```

## Manual test request (run from the repo root)

Calls `Charge` and returns a mock transaction ID as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"amount": {"currency_code": "USD", "units": 10, "nanos": 0}, "credit_card": {"credit_card_number": "4432-8015-6152-0454", "credit_card_cvv": 672, "credit_card_expiration_year": 2030, "credit_card_expiration_month": 1}}' \
  localhost:50052 hipstershop.PaymentService/Charge
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`. Per-request logs are `debug`, so the default only shows startup, shutdown, each card charge, warnings and errors; `compose.yaml` sets `debug` for development.

## Testing

Use `dorny/test-reporter` action (`java-junit`, via `jest-junit`). **Blocking**: the CI fails if any test fails.

```bash
docker run --rm -v "$(pwd):/app" -w /app node:24.21.0-alpine \
  sh -c 'npm ci && npm test'
```

## Coverage

Use `ArtiomTr/jest-coverage-report-action` action. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker run --rm -v "$(pwd):/app" -w /app node:24.21.0-alpine \
  sh -c 'npm ci && npm run test:coverage'
```

## Linting

Use `reviewdog/action-eslint` action, which annotates the PR inline. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app node:24.21.0-alpine \
  sh -c 'npm ci && npm run lint'
```

## Formatting

Formats the code in place with Prettier (`.prettierrc.json`). CI runs `npx prettier --check "*.js"` instead, which only lists the files that need formatting. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app node:24.21.0-alpine \
  sh -c 'npm ci && npm run format'
```

## Vulnerability scan

Scans the production image built in [Build and run in production](#build-and-run-in-production) for vulnerabilities and secrets with Trivy. CI runs the same command on the image built for the PR: **non-blocking**, informative only. The CD pipeline runs it with `--severity CRITICAL --exit-code 1` on the image pushed to the registry: **blocking** for promotion to the hardened scenario (any critical finding stops it), informative only for the baseline one.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret paymentservice:prod
```
