# emailservice

The Email service logs a request to send an order confirmation email. It currently always runs in dummy mode (no email is actually sent).

## Development (hot reload, run from the repo root)

```bash
docker compose up emailservice
```

## Build and run in production

```bash
docker build -t emailservice:prod .
docker run --rm -p 8080:8080 -e PORT=8080 emailservice:prod
```

## Manual test request (run from the repo root)

Calls `SendOrderConfirmation` and returns an empty response as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"email": "someone@example.com", "order": {"order_id": "123", "shipping_tracking_id": "TRACK-1"}}' \
  localhost:8080 hipstershop.EmailService/SendOrderConfirmation
```

## Testing

Use `dorny/test-reporter` action (`java-junit`, via `pytest --junitxml`). **Blocking**: the CI fails if any test fails.

```bash
docker run --rm -v "$(pwd):/app" -w /app python:3.14.7-alpine \
  sh -c 'pip install -r requirements.txt -r requirements-test.in && pytest --junitxml=reports/junit.xml'
```

## Coverage

Use `irongut/CodeCoverageSummary` action (Cobertura XML, via `pytest-cov`). `demo_pb2*.py` excluded as it's generated code. **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker run --rm -v "$(pwd):/app" -w /app python:3.14.7-alpine \
  sh -c 'pip install -r requirements.txt -r requirements-test.in && pytest --cov=email_server --cov=logger --cov-report=term --cov-report=xml:reports/coverage.xml'
```

## Linting

Use `astral-sh/ruff-action`, which runs the linter itself and annotates the PR natively (no separate report file needed). `demo_pb2*.py` excluded as it's generated code. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app python:3.14.7-alpine \
  sh -c 'pip install -r requirements-test.in && ruff check . --exclude demo_pb2.py,demo_pb2_grpc.py'
```

## Formatting

Formats the code in place with `ruff format`. `demo_pb2*.py` excluded as it's generated code. CI runs it with `--check` instead, which only lists the files that need formatting. **Non-blocking**: informative only, never fails the CI.

```bash
docker run --rm -v "$(pwd):/app" -w /app python:3.14.7-alpine \
  sh -c 'pip install -r requirements-test.in && ruff format . --exclude demo_pb2.py,demo_pb2_grpc.py'
```

## Vulnerability scan

Scans the production image built in [Build and run in production](#build-and-run-in-production) for vulnerabilities and secrets with Trivy. CI runs the same command on the image built for the PR: **non-blocking**, informative only. The CD pipeline runs it with `--severity CRITICAL --exit-code 1` on the image pushed to the registry: **blocking** for promotion to the hardened scenario (any critical finding stops it), informative only for the baseline one.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret emailservice:prod
```
