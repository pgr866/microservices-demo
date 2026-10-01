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

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`. Per-request logs are `debug`, so the default only shows startup, shutdown, warnings and errors; `compose.yaml` sets `debug` for development.

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

## Formatting

Formats the code in place with google-java-format. Generated code excluded as only `src/` is formatted. CI runs it with `--dry-run --set-exit-if-changed` instead of `--replace`, which only lists the files that need formatting. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app eclipse-temurin:25.0.4.1_1-jdk-alpine \
  sh -c 'wget -qO /tmp/google-java-format.jar https://github.com/google/google-java-format/releases/download/v1.37.0/google-java-format-1.37.0-all-deps.jar \
    && java --add-exports=jdk.compiler/com.sun.tools.javac.api=ALL-UNNAMED \
      --add-exports=jdk.compiler/com.sun.tools.javac.code=ALL-UNNAMED \
      --add-exports=jdk.compiler/com.sun.tools.javac.file=ALL-UNNAMED \
      --add-exports=jdk.compiler/com.sun.tools.javac.parser=ALL-UNNAMED \
      --add-exports=jdk.compiler/com.sun.tools.javac.tree=ALL-UNNAMED \
      --add-exports=jdk.compiler/com.sun.tools.javac.util=ALL-UNNAMED \
      -jar /tmp/google-java-format.jar --replace $(find src -name "*.java")'
```

## Vulnerability scan

Scans the production image built in [Build and run in production](#build-and-run-in-production) for vulnerabilities and secrets with Trivy. CI runs the same command on the image built for the PR: **non-blocking**, informative only. The CD pipeline runs it with `--severity CRITICAL --exit-code 1` on the image pushed to the registry: **blocking** for promotion to the hardened scenario (any critical finding stops it), informative only for the baseline one.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret adservice:prod
```
