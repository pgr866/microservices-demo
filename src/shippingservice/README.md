# shippingservice

The Shipping service provides price quote, tracking IDs, and the impression of order fulfillment & shipping processes.

## Development (hot reload, run from the repo root)

```bash
docker compose up shippingservice
```

## Build and run in production

```bash
docker build -t shippingservice:prod .
docker run --rm -p 50051:50051 -e PORT=50051 shippingservice:prod
```

## Manual test request (run from the repo root)

Calls `GetQuote` and returns a shipping cost quote as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"address": {"street_address": "1600 Amphitheatre Parkway", "city": "Mountain View", "state": "CA", "country": "USA", "zip_code": 94043}, "items": [{"product_id": "OLJCESPC7Z", "quantity": 1}]}' \
  localhost:50051 hipstershop.ShippingService/GetQuote
```

## Testing

Use `dorny/test-reporter` action (`golang-json`). **Blocking**: the CI fails if any test fails.

```bash
docker run --rm -v "$(pwd):/app" -w /app golang:1.27.1-alpine \
  sh -c 'go test -json ./... > test-report.json'
```

## Coverage

Use `gwatts/go-coverage-action` action. `genproto/` excluded as it's generated code. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker run --rm -v "$(pwd):/app" -w /app golang:1.27.1-alpine \
  sh -c 'go test -coverprofile=coverage.out $(go list ./... | grep -v /genproto) && go tool cover -func=coverage.out'
```

## Linting

Use `golangci-lint-action`, which runs the linter itself and annotates the PR natively (no separate report file needed). **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app golangci/golangci-lint:v2.14.0-alpine \
  golangci-lint run ./...
```

## Formatting

Formats the code in place with `gofmt`. CI runs `gofmt -l .` instead, which only lists the files that need formatting. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app golang:1.27.1-alpine \
  gofmt -l -w .
```

## Vulnerability scan

Scans the production image built in [Build and run in production](#build-and-run-in-production) for vulnerabilities and secrets with Trivy. CI runs the same command on the image built for the PR: **non-blocking**, informative only. The CD pipeline runs it with `--severity CRITICAL --exit-code 1` on the image pushed to the registry: **blocking** for promotion to the hardened scenario (any critical finding stops it), informative only for the baseline one.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret shippingservice:prod
```
