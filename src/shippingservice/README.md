# shippingservice

The Shipping service provides price quote, tracking IDs, and the impression of order fulfillment & shipping processes.

## Build and run in production

```bash
docker build -t shippingservice:prod .
docker run --rm -p 50051:50051 -e PORT=50051 shippingservice:prod
```

## Development

The recommended way is the service's devcontainer (`.devcontainer/shippingservice`): in VS Code, run *Dev Containers: Reopen in Container* and select `shippingservice`. It starts the service with hot reload and gives you every tool used in this document.

The Docker commands of the following sections are stages of `Dockerfile.dev`; run every command from the service folder, `src/shippingservice`.

Without the devcontainer, start it with hot reload using Compose **from the repo root**:

```bash
docker compose up shippingservice
```

## Manual test request

Calls `GetQuote` and returns a shipping cost quote as JSON:

```bash
docker exec shippingservice grpcurl -plaintext -import-path /protos -proto demo.proto \
  -d '{"address": {"street_address": "1600 Amphitheatre Parkway", "city": "Mountain View", "state": "CA", "country": "USA", "zip_code": 94043}, "items": [{"product_id": "OLJCESPC7Z", "quantity": 1}]}' \
  localhost:50051 hipstershop.ShippingService/GetQuote
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.
- `PORT`: port to listen on (default `50051`).

## Testing

Use `dorny/test-reporter` action (`golang-json`). **Blocking**: the CI fails if any test fails.

```bash
docker build -f Dockerfile.dev --target test --output type=local,dest=. .
```

## Coverage

Use `gwatts/go-coverage-action` action. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker build -f Dockerfile.dev --target coverage --output type=local,dest=. .
```

## Linting

Use `golangci/golangci-lint-action`, which runs the linter itself and annotates the PR natively (no separate report file needed). **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target lint .
```

## Formatting

Formats the code in place with `gofmt`. **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target format --output type=local,dest=. .
docker build -f Dockerfile.dev --target format-check .
```

## Dependencies

Updates every module to its latest version and tidies `go.mod`/`go.sum`.

```bash
docker build -f Dockerfile.dev --target dependencies --output type=local,dest=. .
```

## Generated code

`genproto/` is generated from `protos/demo.proto` by `genproto.sh`. Regenerate it after changing the `.proto`:

```bash
docker build -f Dockerfile.dev --target codegen --build-context protos=../../protos --output type=local,dest=. .
```

## Vulnerability scan

Builds the production image and scans it for vulnerabilities and secrets with Trivy, and the Go modules with `govulncheck`. CI runs both **non-blocking**; the CD pipeline runs Trivy with `--severity CRITICAL --exit-code 1`, **blocking** the promotion to the hardened scenario.

```bash
docker build -t shippingservice:prod .
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.76.0 \
  image --scanners vuln,secret shippingservice:prod
```

```bash
docker build -f Dockerfile.dev --target vulncheck .
```
