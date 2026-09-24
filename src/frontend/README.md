# frontend

## Testing

    go test -v ./...
    go test -coverprofile=coverage.out $(go list ./... | grep -v /genproto)
    go tool cover -func=coverage.out
    go tool cover -html=coverage.out -o coverage.html

## Linting

    golangci-lint run ./...
