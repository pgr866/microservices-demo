# emailservice

The Email service logs a request to send an order confirmation email. It currently always runs in dummy mode (no email is actually sent).

## Development (hot reload, run from the repo root)

The commands of the following sections are stages of `Dockerfile.dev`, run from this folder.

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

## Configuration

- `LOG_LEVEL`: `debug`, `info` (default), `warn` or `error`; per-request logs are `debug`, and `compose.yaml` sets `debug` for development.

## Testing

Use `dorny/test-reporter` action (`java-junit`, via `pytest --junitxml`). **Blocking**: the CI fails if any test fails.

```bash
docker build -f Dockerfile.dev --target test --output type=local,dest=. .
```

## Coverage

Use `irongut/CodeCoverageSummary` action (Cobertura XML, via `pytest-cov`). **Blocking**: the CI fails if coverage drops below the configured threshold.

```bash
docker build -f Dockerfile.dev --target coverage --output type=local,dest=. .
```

## Linting

Use `astral-sh/ruff-action`, which runs the linter itself and annotates the PR natively (no separate report file needed). **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target lint .
```

## Formatting

Formats the code in place with `ruff format`. **Non-blocking**: informative only, never fails the CI.

```bash
docker build -f Dockerfile.dev --target format --output type=local,dest=. .
docker build -f Dockerfile.dev --target format-check .
```

## Dependencies

Regenerates `requirements.txt` from `requirements.in` with the latest versions; the direct dependencies are pinned by hand in the `.in` files.

```bash
docker build -f Dockerfile.dev --target dependencies --output type=local,dest=. .
```

## Generated code

`demo_pb2.py` and `demo_pb2_grpc.py` are generated from `protos/demo.proto` by `genproto.sh`. Regenerate them after changing the `.proto`:

```bash
docker build -f Dockerfile.dev --target codegen --build-context protos=../../protos --output type=local,dest=. .
```

## Vulnerability scan

Scans the production image built above for vulnerabilities and secrets with Trivy. CI runs it **non-blocking**; the CD pipeline runs it with `--severity CRITICAL --exit-code 1`, **blocking** the promotion to the hardened scenario.

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:0.75.0 \
  image --scanners vuln,secret emailservice:prod
```
