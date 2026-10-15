# currencyservice

The Currency service converts amounts between currencies using a fixed conversion table.

## Build and run in production

```bash
docker build -t currencyservice:prod .
docker run --rm -p 7000:7000 -e PORT=7000 currencyservice:prod
```

## Development

The recommended way is the service's devcontainer (`.devcontainer/currencyservice`): in VS Code, run *Dev Containers: Reopen in Container* and select `currencyservice`. It starts the service with hot reload and gives you every tool used in this document.

After changing `package.json`, run `docker compose down -v currencyservice` first so the `node_modules` volume is refilled.

The Docker commands of the following sections are stages of `Dockerfile.dev`; run every command from the service folder, `src/currencyservice`.

Without the devcontainer, start it with hot reload using Compose **from the repo root**:

```bash
docker compose up currencyservice
```

## Manual test request

Calls `Convert` and returns the converted amount as JSON:

```bash
docker exec currencyservice grpcurl -plaintext -import-path /protos -proto demo.proto \
  -d '{"from": {"currency_code": "USD", "units": 10, "nanos": 0}, "to_code": "EUR"}' \
  localhost:7000 hipstershop.CurrencyService/Convert
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.
- `PORT`: port to listen on (default `7000`).

## Testing

Use `dorny/test-reporter` action (`java-junit`, via `jest-junit`). **Blocking**: the CI fails if any test fails.

```bash
docker build -f Dockerfile.dev --target test --output type=local,dest=. .
```

## Coverage

Use `ArtiomTr/jest-coverage-report-action` action. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker build -f Dockerfile.dev --target coverage --output type=local,dest=. .
```

## Linting

Use `reviewdog/action-eslint` action, which annotates the PR inline. **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target lint .
```

## Formatting

Formats the code in place with Prettier (`.prettierrc.json`). **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target format --output type=local,dest=. .
docker build -f Dockerfile.dev --target format-check .
```

## Dependencies

The command lists the outdated dependencies and the known vulnerabilities and refreshes `package-lock.json`; to update one, change its version in `package.json` first.

```bash
docker build -f Dockerfile.dev --target dependencies --output type=local,dest=. .
```

## Generated code

`proto/` is a copy of `protos/` that `@grpc/proto-loader` reads at runtime. Refresh it with `genproto.sh` after changing the `.proto`:

```bash
docker build -f Dockerfile.dev --target codegen --build-context protos=../../protos --output type=local,dest=. .
```

## Vulnerability scan

Builds the production image and scans it for vulnerabilities and secrets with Trivy. CI runs it **non-blocking**; the CD pipeline runs it with `--severity CRITICAL --exit-code 1`, **blocking** the promotion to the hardened scenario.

```bash
docker build -t currencyservice:prod .
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.76.0 \
  image --scanners vuln,secret currencyservice:prod
```
