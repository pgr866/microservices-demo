# adservice

The Ad service provides advertisements based on context keys. If no context keys are provided, or none match a known category, it returns random ads.

## Development (hot reload, run from the repo root)

The commands of the following sections are stages of `Dockerfile.dev`, run from this folder.

```bash
docker compose up adservice
```

## Build and run in production

```bash
docker build -t adservice:prod .
docker run --rm -p 9555:9555 -e PORT=9555 adservice:prod
```

## Manual test request (run from the repo root)

Calls `GetAds` with a context key and returns the matching ads as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"context_keys": ["kitchen"]}' \
  localhost:9555 hipstershop.AdService/GetAds
```

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.

## Testing

Use `dorny/test-reporter` action (`java-junit`, reading `build/test-results/test/*.xml`). **Blocking**: the CI fails if any test fails.

```bash
docker build -f Dockerfile.dev --target test --output type=local,dest=. .
```

## Coverage

Use `madrapps/jacoco-report` action (JaCoCo XML, `build/reports/jacoco/test/jacocoTestReport.xml`). **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker build -f Dockerfile.dev --target coverage --output type=local,dest=. .
```

## Linting

Use `reviewdog/action-setup` + `reviewdog -f=checkstyle` (Checkstyle XML, `build/reports/checkstyle/*.xml`) to annotate the PR. **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target lint .
```

## Formatting

Formats the code in place with google-java-format. **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target format --output type=local,dest=. .
docker build -f Dockerfile.dev --target format-check .
```

## Dependencies

The command shows the resolved dependency tree and updates the Gradle wrapper; versions are declared at the top of `build.gradle`.

```bash
docker build -f Dockerfile.dev --target dependencies --output type=local,dest=. .
```

## Generated code

`src/main/proto/demo.proto` is a copy of `protos/demo.proto` from which Gradle generates the Java code on every build. Refresh it with `genproto.sh` after changing the `.proto`:

```bash
docker build -f Dockerfile.dev --target codegen --build-context protos=../../protos --output type=local,dest=. .
```

## Vulnerability scan

Scans the production image built above for vulnerabilities and secrets with Trivy. CI runs it **non-blocking**; the CD pipeline runs it with `--severity CRITICAL --exit-code 1`, **blocking** the promotion to the hardened scenario.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret adservice:prod
```
