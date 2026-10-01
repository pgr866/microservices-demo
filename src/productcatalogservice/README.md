# productcatalogservice

The Product Catalog service provides the list of products, product details, and search functionality for the online store.

## Development (hot reload, run from the repo root)

```bash
docker compose up productcatalogservice
```

## Build and run in production

```bash
docker build -t productcatalogservice:prod .
docker run --rm -p 3550:3550 -e PORT=3550 productcatalogservice:prod
```

## Manual test request (run from the repo root)

Calls `ListProducts` and returns the product catalog as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto localhost:3550 hipstershop.ProductCatalogService/ListProducts
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`. Per-request logs are `debug`, so the default only shows startup, shutdown, warnings and errors; `compose.yaml` sets `debug` for development.

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
  image --scanners vuln,secret productcatalogservice:prod
```

## Dynamic catalog reloading / artificial delay

This service has a "dynamic catalog reloading" feature that is purposefully
not well implemented. The goal of this feature is to allow you to modify the
`products.json` file and have the changes be picked up without having to
restart the service.

However, this feature is bugged: the catalog is actually reloaded on each
request, introducing a noticeable delay in the frontend. This delay will also
show up in profiling tools: the `parseCatalog` function will take more than 80%
of the CPU time.

You can trigger this feature (and the delay) by sending a `USR1` signal and
remove it (if needed) by sending a `USR2` signal:

```
# Trigger bug
kubectl exec \
    $(kubectl get pods -l app=productcatalogservice -o jsonpath='{.items[0].metadata.name}') \
    -c server -- kill -USR1 1
# Remove bug
kubectl exec \
    $(kubectl get pods -l app=productcatalogservice -o jsonpath='{.items[0].metadata.name}') \
    -c server -- kill -USR2 1
```

## Latency injection

This service has an `EXTRA_LATENCY` environment variable. This will inject a sleep for the specified [time.Duration](https://golang.org/pkg/time/#ParseDuration) on every call to
to the server.

For example, use `EXTRA_LATENCY="5.5s"` to sleep for 5.5 seconds on every request.
