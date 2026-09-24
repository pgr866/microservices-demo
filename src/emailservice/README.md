# Email Service

The Email service logs a request to send an order confirmation email. It currently always runs in dummy mode (no email is actually sent).

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
