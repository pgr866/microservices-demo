# checkoutservice

## Testing

```bash
go test -v ./...

# Run tests and record which lines executed into coverage.out
# (genproto/ is generated code, excluded so it doesn't skew the numbers)
go test -coverprofile=coverage.out $(go list ./... | grep -v /genproto)
# Print a per-function coverage summary in the terminal
go tool cover -func=coverage.out
# Generate an HTML report with covered/uncovered lines highlighted
go tool cover -html=coverage.out -o coverage.html
```

## Linting

```bash
golangci-lint run ./...
```
