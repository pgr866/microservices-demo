# frontend

The Frontend service is the web shop: an HTTP server that renders the pages (catalog, product, cart, checkout) and calls the other services over gRPC.

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

## Development

The recommended way is the service's devcontainer (`.devcontainer/frontend`): in VS Code, run *Dev Containers: Reopen in Container* and select `frontend`. It starts the service with hot reload and gives you every tool used in this document.

The Docker commands of the following sections are stages of `Dockerfile.dev`; run every command from the service folder, `src/frontend`.

Without the devcontainer, start it with hot reload using Compose **from the repo root**:

```bash
docker compose up frontend
```

Also starts `productcatalogservice`, `currencyservice`, `cartservice` (and `redis-cart`), `recommendationservice`, `shippingservice`, `checkoutservice` (and its dependencies) and `adservice`, which it depends on.

## Manual test request

Calls the health endpoint and the home page:

```bash
docker exec frontend curl -s http://localhost:8082/_healthz
docker exec frontend curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8082/
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.
- `PORT`: port to listen on (default `8082`).
- `LISTEN_ADDR`: address to bind to (default empty, all interfaces).
- `REQUEST_TIMEOUT`: Go duration after which each request is cut, propagated down the call chain (default `5s`).
- `BASE_URL`: path prefix for redirects, when served under a subpath (default empty).
- `ENV_PLATFORM`: `local` (default), `gcp`, `azure`, `aws`, `onprem` or `alibaba`; only changes the banner.
- `FRONTEND_MESSAGE`: optional banner text.
- `CYMBAL_BRANDING`: `true` for the alternative branding (default `false`).
- `ENABLE_SINGLE_SHARED_SESSION`: `true` makes every visitor share one session (default `false`).
- `AD_SERVICE_ADDR`, `CART_SERVICE_ADDR`, `CHECKOUT_SERVICE_ADDR`, `CURRENCY_SERVICE_ADDR`, `PRODUCT_CATALOG_SERVICE_ADDR`, `RECOMMENDATION_SERVICE_ADDR`, `SHIPPING_SERVICE_ADDR`: `host:port` of each service it calls (required).

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
docker build -t frontend:prod .
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.76.0 \
  image --scanners vuln,secret frontend:prod
```

```bash
docker build -f Dockerfile.dev --target vulncheck .
```
