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

# Run tests and record which lines executed into coverage.out
go test -coverprofile=coverage.out ./...
# Print a per-function coverage summary in the terminal
go tool cover -func=coverage.out
# Generate an HTML report with covered/uncovered lines highlighted
go tool cover -html=coverage.out -o coverage.html
```

## Linting

```bash
golangci-lint run ./...
```
