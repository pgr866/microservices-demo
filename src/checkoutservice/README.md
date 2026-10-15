# checkoutservice

The Checkout service retrieves the user cart, prepares the order and orchestrates the payment, shipping and the email notification.

## Build and run in production

Needs its six dependencies reachable on their `localhost` ports (e.g. `docker compose up -d --wait shippingservice productcatalogservice cartservice currencyservice emailservice paymentservice` from the repo root).

```bash
docker build -t checkoutservice:prod .
docker run --rm --network host -e PORT=5050 \
  -e SHIPPING_SERVICE_ADDR=localhost:50051 -e PRODUCT_CATALOG_SERVICE_ADDR=localhost:3550 \
  -e CART_SERVICE_ADDR=localhost:7070 -e CURRENCY_SERVICE_ADDR=localhost:7000 \
  -e EMAIL_SERVICE_ADDR=localhost:8080 -e PAYMENT_SERVICE_ADDR=localhost:50052 \
  checkoutservice:prod
```

## Development

The recommended way is the service's devcontainer (`.devcontainer/checkoutservice`): in VS Code, run *Dev Containers: Reopen in Container* and select `checkoutservice`. It starts the service with hot reload and gives you every tool used in this document.

The Docker commands of the following sections are stages of `Dockerfile.dev`; run every command from the service folder, `src/checkoutservice`.

Without the devcontainer, start it with hot reload using Compose **from the repo root**:

```bash
docker compose up checkoutservice
```

Also starts `shippingservice`, `productcatalogservice`, `cartservice` (and `redis-cart`), `currencyservice`, `emailservice` and `paymentservice`, which it depends on.

## Manual test request

Adds a product to the cart of `user-1` (`cartservice`), then calls `PlaceOrder` and returns the placed order as JSON:

```bash
docker exec checkoutservice grpcurl -plaintext -import-path /protos -proto demo.proto \
  -d '{"user_id": "user-1", "item": {"product_id": "OLJCESPC7Z", "quantity": 2}}' \
  cartservice:7070 hipstershop.CartService/AddItem

docker exec checkoutservice grpcurl -plaintext -import-path /protos -proto demo.proto \
  -d '{"user_id": "user-1", "user_currency": "EUR", "email": "someone@example.com", "address": {"street_address": "1600 Amphitheatre Parkway", "city": "Mountain View", "state": "CA", "country": "USA", "zip_code": 94043}, "credit_card": {"credit_card_number": "4432801561520454", "credit_card_cvv": 672, "credit_card_expiration_year": 2030, "credit_card_expiration_month": 1}}' \
  localhost:5050 hipstershop.CheckoutService/PlaceOrder
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.
- `PORT`: port to listen on (default `5050`).
- `CART_SERVICE_ADDR`, `CURRENCY_SERVICE_ADDR`, `EMAIL_SERVICE_ADDR`, `PAYMENT_SERVICE_ADDR`, `PRODUCT_CATALOG_SERVICE_ADDR`, `SHIPPING_SERVICE_ADDR`: `host:port` of each service it calls (required).

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

Builds the production image and scans it with Trivy, for vulnerabilities and secrets and for misconfigurations in its build instructions, and the Go modules with `govulncheck`. CI runs both **non-blocking**; the CD pipeline runs Trivy with `--severity CRITICAL --exit-code 1`, **blocking** the promotion to the hardened scenario.

```bash
docker build -t checkoutservice:prod .
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.76.0 \
  image --scanners vuln,secret --image-config-scanners misconfig checkoutservice:prod
```

```bash
docker build -f Dockerfile.dev --target vulncheck .
```
