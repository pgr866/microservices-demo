# adservice

The Ad service provides advertisements based on context keys. If no context keys are provided, or none match a known category, it returns random ads.

## Development (hot reload, run from the repo root)

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

## Testing

Use `dorny/test-reporter` action (`java-junit`, reading `build/test-results/test/*.xml`). **Blocking**: the CI fails if any test fails.

```bash
docker run --rm -v "$(pwd):/app" -w /app eclipse-temurin:25.0.4.1_1-jdk-alpine \
  sh -c 'apk add --no-cache gcompat && ./gradlew test --no-daemon'
```

## Coverage

Use `madrapps/jacoco-report` action (JaCoCo XML, `build/reports/jacoco/test/jacocoTestReport.xml`). Protobuf/gRPC generated classes excluded as they're generated code. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker run --rm -v "$(pwd):/app" -w /app eclipse-temurin:25.0.4.1_1-jdk-alpine \
  sh -c 'apk add --no-cache gcompat && ./gradlew jacocoTestReport --no-daemon'
```

## Linting

Use `reviewdog/action-setup` + `reviewdog -f=checkstyle` (Checkstyle XML, `build/reports/checkstyle/*.xml`) to annotate the PR. Generated code excluded as only `src/*/java` is linted. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app eclipse-temurin:25.0.4.1_1-jdk-alpine \
  sh -c 'apk add --no-cache gcompat && ./gradlew checkstyleMain checkstyleTest --continue --no-daemon'
```
