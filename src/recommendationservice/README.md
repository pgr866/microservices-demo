# Recommendation Service

The Recommendation service returns a list of product recommendations, excluding whatever products are already in the request.

## Testing

```bash
pip install -r requirements.txt -r requirements-test.in
pytest
pytest --cov
```

## Linting

```bash
ruff check . --exclude demo_pb2.py,demo_pb2_grpc.py
```
