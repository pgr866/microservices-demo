# productcatalogservice

The Product Catalog service provides the list of products, product details, and search functionality for the online store.

## Build and run in production

```bash
docker build -t productcatalogservice:prod .
docker run --rm -p 3550:3550 -e PORT=3550 productcatalogservice:prod
```

## Development

The recommended way is the service's devcontainer (`.devcontainer/productcatalogservice`): in VS Code, run *Dev Containers: Reopen in Container* and select `productcatalogservice`. It starts the service with hot reload and gives you every tool used in this document.

The Docker commands of the following sections are stages of `Dockerfile.dev`; run every command from the service folder, `src/productcatalogservice`.

Without the devcontainer, start it with hot reload using Compose **from the repo root**:

```bash
docker compose up productcatalogservice
```

## Manual test request

Calls `ListProducts` and returns the product catalog as JSON:

```bash
docker exec productcatalogservice grpcurl -plaintext -import-path /protos -proto demo.proto \
  localhost:3550 hipstershop.ProductCatalogService/ListProducts
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.
- `PORT`: port to listen on (default `3550`).
- `EXTRA_LATENCY`: Go duration (e.g. `200ms`) added to every request, to simulate a slow service (default none).

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
docker build -t productcatalogservice:prod .
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.76.0 \
  image --scanners vuln,secret productcatalogservice:prod
```

```bash
docker build -f Dockerfile.dev --target vulncheck .
```
