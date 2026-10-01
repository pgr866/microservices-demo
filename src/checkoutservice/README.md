# checkoutservice

The Checkout service retrieves the user cart, prepares the order and orchestrates the payment, shipping and the email notification.

## Development (hot reload, run from the repo root)

Also starts `shippingservice`, `productcatalogservice`, `cartservice` (and `redis-cart`), `currencyservice`, `emailservice` and `paymentservice`, which it depends on.

```bash
docker compose up checkoutservice
```

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

## Manual test request (run from the repo root)

Adds a product to the cart of `user-1` (`cartservice`), then calls `PlaceOrder` and returns the placed order as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"user_id": "user-1", "item": {"product_id": "OLJCESPC7Z", "quantity": 2}}' \
  localhost:7070 hipstershop.CartService/AddItem

docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"user_id": "user-1", "user_currency": "EUR", "email": "someone@example.com", "address": {"street_address": "1600 Amphitheatre Parkway", "city": "Mountain View", "state": "CA", "country": "USA", "zip_code": 94043}, "credit_card": {"credit_card_number": "4432801561520454", "credit_card_cvv": 672, "credit_card_expiration_year": 2030, "credit_card_expiration_month": 1}}' \
  localhost:5050 hipstershop.CheckoutService/PlaceOrder
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`. Per-request logs are `debug`, so the default only shows startup, shutdown, each card charge, warnings and errors; `compose.yaml` sets `debug` for development.

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
  image --scanners vuln,secret checkoutservice:prod
```
