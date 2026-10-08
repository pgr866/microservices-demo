# cartservice

The Cart service stores the items in a user's shopping cart in Redis (falling back to an in-memory store if `REDIS_ADDR` isn't set) and exposes it via gRPC.

## Development (hot reload, run from the repo root)

Also starts `redis-cart`, the same Redis it uses in the cluster.

The commands of the following sections are stages of `Dockerfile.dev`, run from this folder.

```bash
docker compose up cartservice
```

## Build and run in production

```bash
docker build -t cartservice:prod .
docker run --rm -p 7070:7070 -e ASPNETCORE_HTTP_PORTS=7070 cartservice:prod
```

## Manual test request (run from the repo root)

Calls `AddItem` to put a product in a user's cart, then `GetCart` to return that cart as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"user_id": "user-1", "item": {"product_id": "OLJCESPC7Z", "quantity": 2}}' \
  localhost:7070 hipstershop.CartService/AddItem

docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"user_id": "user-1"}' \
  localhost:7070 hipstershop.CartService/GetCart
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.

## Testing

Use `dorny/test-reporter` action (`dotnet-trx`, reading `tests/TestResults/test-results.trx`). **Blocking**: the CI fails if any test fails.

```bash
docker build -f Dockerfile.dev --target test --output type=local,dest=. .
```

## Coverage

Use `irongut/CodeCoverageSummary` action (Cobertura XML, `tests/TestResults/*/coverage.cobertura.xml`). **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker build -f Dockerfile.dev --target coverage --output type=local,dest=. .
```

## Linting

Use `reviewdog/action-setup` + `reviewdog -f=dotnet` (`dotnet format` output piped in) to annotate the PR. **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target lint .
```

## Formatting

Formats the code in place with `dotnet format`, the same tool the linter runs with `--verify-no-changes`.

```bash
docker build -f Dockerfile.dev --target format --output type=local,dest=. .
```

## Dependencies

Lists the NuGet packages that are outdated, vulnerable or deprecated; to update one, change its version in the `.csproj`.

```bash
docker build -f Dockerfile.dev --target dependencies .
```

## Generated code

`src/protos/Cart.proto` is the cart part of `protos/demo.proto`, from which `Grpc.Tools` generates the C# code on every build. After changing the `.proto`, copy the change there by hand.

## Vulnerability scan

Scans the production image built above for vulnerabilities and secrets with Trivy. CI runs it **non-blocking**; the CD pipeline runs it with `--severity CRITICAL --exit-code 1`, **blocking** the promotion to the hardened scenario.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret cartservice:prod
```
