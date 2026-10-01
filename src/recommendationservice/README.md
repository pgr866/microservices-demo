# recommendationservice

The Recommendation service returns up to five random products from the product catalog, excluding whatever products are already in the request.

## Development (hot reload, run from the repo root)

Also starts `productcatalogservice`, which it depends on.

```bash
docker compose up recommendationservice
```

## Build and run in production

Needs `productcatalogservice` reachable on `localhost:3550` (e.g. `docker compose up -d --wait productcatalogservice` from the repo root).

```bash
docker build -t recommendationservice:prod .
docker run --rm --network host -e PORT=8081 -e PRODUCT_CATALOG_SERVICE_ADDR=localhost:3550 recommendationservice:prod
```

## Manual test request (run from the repo root)

Calls `ListRecommendations` and returns the recommended product IDs as JSON:

```bash
docker run --rm --network host -v "$(pwd)/protos:/protos" -w /protos fullstorydev/grpcurl:v1.9.3-alpine \
  -plaintext -proto demo.proto -d '{"user_id": "user-1", "product_ids": ["OLJCESPC7Z"]}' \
  localhost:8081 hipstershop.RecommendationService/ListRecommendations
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
  sh -c 'pip install -r requirements.txt -r requirements-test.in && pytest --cov=recommendation_server --cov=logger --cov-report=term --cov-report=xml:reports/coverage.xml'
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
  image --scanners vuln,secret recommendationservice:prod
```
