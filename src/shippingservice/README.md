# Shipping Service

The Shipping service provides price quote, tracking IDs, and the impression of order fulfillment & shipping processes.

## Build

From `src/shippingservice`, run:

```
docker build ./
```

## Testing

```bash
go test -v ./...

go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Linting

```bash
golangci-lint run ./...
```
