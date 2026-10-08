import json
import logging
import signal
from unittest import mock

import grpc
import pytest
from grpc_health.v1 import health_pb2

import demo_pb2
import email_server
from email_server import DummyEmailService
from logger import CustomJsonFormatter, getJSONLogger


def make_item(product_id, quantity, units, nanos):
    return demo_pb2.OrderItem(
        item=demo_pb2.CartItem(product_id=product_id, quantity=quantity),
        cost=demo_pb2.Money(currency_code="USD", units=units, nanos=nanos),
    )


def build_order(items):
    return demo_pb2.OrderResult(
        order_id="123",
        shipping_tracking_id="TRACK-1",
        shipping_cost=demo_pb2.Money(currency_code="USD", units=5, nanos=990000000),
        shipping_address=demo_pb2.Address(
            street_address="1600 Amphitheatre Pkwy",
            city="Mountain View",
            country="USA",
            zip_code=94043,
        ),
        items=items,
    )


def test_check_reports_serving():
    response = DummyEmailService().Check(health_pb2.HealthCheckRequest(), None)
    assert response.status == health_pb2.HealthCheckResponse.SERVING


def test_watch_aborts_as_unimplemented():
    context = mock.Mock()
    DummyEmailService().Watch(health_pb2.HealthCheckRequest(), context)
    context.abort.assert_called_once_with(
        grpc.StatusCode.UNIMPLEMENTED, "Watch is not implemented"
    )


def test_send_order_confirmation_returns_empty():
    request = demo_pb2.SendOrderConfirmationRequest(
        email="someone@example.com",
        order=build_order([make_item("OLJCESPC7Z", 1, 19, 990000000)]),
    )
    assert DummyEmailService().SendOrderConfirmation(request, None) == demo_pb2.Empty()


def format_record(**extra):
    record = logging.LogRecord(
        "test", logging.WARNING, __file__, 1, "hello", None, None
    )
    record.__dict__.update(extra)
    formatter = CustomJsonFormatter("%(timestamp)s %(severity)s %(name)s %(message)s")
    return json.loads(formatter.format(record))


def test_logger_defaults_severity_and_timestamp():
    log = format_record()
    assert log["severity"] == "WARNING"
    assert log["timestamp"]
    assert log["message"] == "hello"


def test_logger_uppercases_explicit_severity():
    assert format_record(severity="error")["severity"] == "ERROR"


def test_logger_level_comes_from_log_level(monkeypatch):
    monkeypatch.delenv("LOG_LEVEL", raising=False)
    assert getJSONLogger("test-default").level == logging.INFO
    monkeypatch.setenv("LOG_LEVEL", "")
    assert getJSONLogger("test-empty").level == logging.INFO
    monkeypatch.setenv("LOG_LEVEL", "debug")
    assert getJSONLogger("test-debug").level == logging.DEBUG
    monkeypatch.setenv("LOG_LEVEL", "verbose")
    with pytest.raises(ValueError):
        getJSONLogger("test-invalid")


def test_stop_on_signals_stops_the_server_gracefully():
    server = mock.Mock()
    signals = (signal.SIGTERM, signal.SIGINT)
    previous = {s: signal.getsignal(s) for s in signals}
    try:
        email_server.stop_on_signals(server)
        for s in signals:
            # What the interpreter does when the signal arrives.
            signal.getsignal(s)(s, None)
    finally:
        for s, handler in previous.items():
            signal.signal(s, handler)

    assert (
        server.stop.call_args_list
        == [mock.call(email_server.SHUTDOWN_GRACE_SECONDS)] * 2
    )


def test_send_order_confirmation_does_not_log_the_email_address(monkeypatch):
    logged = []
    monkeypatch.setattr(
        email_server.logger, "debug", lambda msg, *a, **k: logged.append(msg)
    )
    request = demo_pb2.SendOrderConfirmationRequest(
        email="someone@example.com", order=demo_pb2.OrderResult(order_id="order-1")
    )

    DummyEmailService().SendOrderConfirmation(request, None)

    assert logged and all("someone@example.com" not in m for m in logged)
    assert any("order-1" in m for m in logged)
