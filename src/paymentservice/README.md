# paymentservice

The Payment service charges the given credit card info (mock) with the given amount and returns a transaction ID.

## Development (hot reload, run from the repo root)

After changing `package.json`, run `docker compose down -v paymentservice` first so the `node_modules` volume is refilled.

The commands of the following sections are stages of `Dockerfile.dev`, run from this folder.

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

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.

## Testing

Use `dorny/test-reporter` action (`java-junit`, via `jest-junit`). **Blocking**: the CI fails if any test fails.

```bash
docker build -f Dockerfile.dev --target test --output type=local,dest=. .
```

## Coverage

Use `ArtiomTr/jest-coverage-report-action` action. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker build -f Dockerfile.dev --target coverage --output type=local,dest=. .
```

## Linting

Use `reviewdog/action-eslint` action, which annotates the PR inline. **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target lint .
```

## Formatting

Formats the code in place with Prettier (`.prettierrc.json`). **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target format --output type=local,dest=. .
docker build -f Dockerfile.dev --target format-check .
```

## Dependencies

The command lists the outdated dependencies and the known vulnerabilities and refreshes `package-lock.json`; to update one, change its version in `package.json` first.

```bash
docker build -f Dockerfile.dev --target dependencies --output type=local,dest=. .
```

## Generated code

`proto/` is a copy of `protos/` that `@grpc/proto-loader` reads at runtime. Refresh it with `genproto.sh` after changing the `.proto`:

```bash
docker build -f Dockerfile.dev --target codegen --build-context protos=../../protos --output type=local,dest=. .
```

## Vulnerability scan

Scans the production image built above for vulnerabilities and secrets with Trivy. CI runs it **non-blocking**; the CD pipeline runs it with `--severity CRITICAL --exit-code 1`, **blocking** the promotion to the hardened scenario.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret paymentservice:prod
```
