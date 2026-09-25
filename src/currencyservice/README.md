# currencyservice

The Currency service converts amounts between currencies using a fixed conversion table.

## Development (hot reload, run from the repo root)

```bash
docker compose up currencyservice
```

## Build and run in production

```bash
docker build -t currencyservice:prod .
docker run --rm -p 7000:7000 -e PORT=7000 currencyservice:prod
```

## Manual test request (run from the repo root)

Calls `Convert` and returns the converted amount as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"from": {"currency_code": "USD", "units": 10, "nanos": 0}, "to_code": "EUR"}' \
  localhost:7000 hipstershop.CurrencyService/Convert
```

## Testing

Use `dorny/test-reporter` action (`java-junit`, via `jest-junit`). **Blocking**: the CI fails if any test fails.

```bash
docker run --rm -v "$(pwd):/usr/src/app" -w /usr/src/app node:24.21.0-alpine \
  sh -c 'npm ci && npm test'
```

## Coverage

Use `ArtiomTr/jest-coverage-report-action` action. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker run --rm -v "$(pwd):/usr/src/app" -w /usr/src/app node:24.21.0-alpine \
  sh -c 'npm ci && npm run test:coverage'
```

## Linting

Use `reviewdog/action-eslint` action, which annotates the PR inline. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/usr/src/app" -w /usr/src/app node:24.21.0-alpine \
  sh -c 'npm ci && npm run lint'
```
