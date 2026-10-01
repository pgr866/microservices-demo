# cartservice

The Cart service stores the items in a user's shopping cart in Redis (falling back to an in-memory store if `REDIS_ADDR` isn't set) and exposes it via gRPC.

## Development (hot reload, run from the repo root)

Also starts `redis-cart`, the same Redis it uses in the cluster.

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

## Testing

Use `dorny/test-reporter` action (`dotnet-trx`, reading `tests/TestResults/test-results.trx`). **Blocking**: the CI fails if any test fails.

```bash
docker run --rm -v "$(pwd):/app" -w /app mcr.microsoft.com/dotnet/sdk:10.0.401-alpine3.24 \
  sh -c 'apk add --no-cache gcompat && dotnet test tests/cartservice.tests.csproj --logger "trx;LogFileName=test-results.trx"'
```

## Coverage

Use `irongut/CodeCoverageSummary` action (Cobertura XML, `tests/TestResults/*/coverage.cobertura.xml`). Protobuf/gRPC generated code (`obj/`) excluded via `tests/coverlet.runsettings` as it's generated code. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker run --rm -v "$(pwd):/app" -w /app mcr.microsoft.com/dotnet/sdk:10.0.401-alpine3.24 \
  sh -c 'apk add --no-cache gcompat && dotnet test tests/cartservice.tests.csproj --collect:"XPlat Code Coverage" --settings tests/coverlet.runsettings'
```

## Linting

Use `reviewdog/action-setup` + `reviewdog -f=dotnet` (`dotnet format` output piped in) to annotate the PR. Generated code excluded as `dotnet format` ignores `obj/` by default. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app mcr.microsoft.com/dotnet/sdk:10.0.401-alpine3.24 \
  sh -c 'apk add --no-cache gcompat && dotnet format --verify-no-changes'
```

## Formatting

Formats the code in place with `dotnet format`, the same tool the linter runs with `--verify-no-changes`, so CI needs no extra step. It also fixes the deliberate linter finding.

```bash
docker run --rm -v "$(pwd):/app" -w /app mcr.microsoft.com/dotnet/sdk:10.0.401-alpine3.24 \
  sh -c 'apk add --no-cache gcompat && dotnet format'
```

## Vulnerability scan

Scans the production image built in [Build and run in production](#build-and-run-in-production) for vulnerabilities and secrets with Trivy. CI runs the same command on the image built for the PR: **non-blocking**, informative only. The CD pipeline runs it with `--severity CRITICAL --exit-code 1` on the image pushed to the registry: **blocking** for promotion to the hardened scenario (any critical finding stops it), informative only for the baseline one.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret cartservice:prod
```
