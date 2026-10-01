# frontend

The Frontend service is the web shop: an HTTP server that renders the pages (catalog, product, cart, checkout) and calls the other services over gRPC.

## Development (hot reload, run from the repo root)

Also starts `productcatalogservice`, `currencyservice`, `cartservice` (and `redis-cart`), `recommendationservice`, `shippingservice`, `checkoutservice` (and its dependencies) and `adservice`, which it depends on. The shop is then at http://localhost:8082.

```bash
docker compose up frontend
```

## Build and run in production

Needs its seven dependencies reachable on their `localhost` ports (e.g. `docker compose up -d --wait productcatalogservice currencyservice cartservice recommendationservice shippingservice checkoutservice adservice` from the repo root).

```bash
docker build -t frontend:prod .
docker run --rm --network host -e PORT=8082 \
  -e PRODUCT_CATALOG_SERVICE_ADDR=localhost:3550 -e CURRENCY_SERVICE_ADDR=localhost:7000 \
  -e CART_SERVICE_ADDR=localhost:7070 -e RECOMMENDATION_SERVICE_ADDR=localhost:8081 \
  -e SHIPPING_SERVICE_ADDR=localhost:50051 -e CHECKOUT_SERVICE_ADDR=localhost:5050 \
  -e AD_SERVICE_ADDR=localhost:9555 \
  frontend:prod
```

## Manual test request

Calls the health endpoint and the home page (or open http://localhost:8082 in a browser):

```bash
docker run --rm --network host curlimages/curl:8.22.0 -s http://localhost:8082/_healthz
docker run --rm --network host curlimages/curl:8.22.0 -s -o /dev/null -w "%{http_code}\n" http://localhost:8082/
```

## Configuration

Optional environment variables, besides the `*_SERVICE_ADDR` ones:

- `REQUEST_TIMEOUT`: deadline of every request, passed on to the gRPC calls (Go duration, default `5s`).
- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`. Per-request logs are `debug`, so the default only shows startup, shutdown, warnings and errors; `compose.yaml` sets `debug` for development.
- `ENV_PLATFORM`: platform badge shown on the page (`local`, `gcp`, `aws`, `azure`, `onprem` or `alibaba`; default `local`).
- `BASE_URL`: path prefix to serve the shop under (e.g. `/shop`).
- `LISTEN_ADDR`: address to listen on (default: all interfaces).
- `CYMBAL_BRANDING`, `FRONTEND_MESSAGE`, `BANNER_COLOR`: branding, message banner and banner color.
- `ENABLE_SINGLE_SHARED_SESSION`: `true` to give every visitor the same session (and cart).

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
  image --scanners vuln,secret frontend:prod
```
