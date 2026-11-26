import json
import logging
import signal
from datetime import datetime, timedelta
from unittest import mock

import grpc
import pytest
from grpc_health.v1 import health_pb2

import demo_pb2
import email_server
from email_server import EmailService
from logger import CustomJsonFormatter, exit_on_invalid_log_level, getJSONLogger

MODULE = email_server


def confirmation_request():
    return demo_pb2.SendOrderConfirmationRequest(
        email="someone@example.com", order=demo_pb2.OrderResult(order_id="order-1")
    )


def test_send_order_confirmation_returns_empty():
    response = EmailService().SendOrderConfirmation(confirmation_request(), None)
    assert response == demo_pb2.Empty()


def test_send_order_confirmation_logs_the_order_id_but_not_the_email(monkeypatch):
    debug = mock.Mock()
    monkeypatch.setattr(email_server.logger, "debug", debug)

    EmailService().SendOrderConfirmation(confirmation_request(), None)

    # Rendered as logging would: the format string with its lazy arguments.
    template, *args = debug.call_args.args
    message = template % tuple(args)
    assert "order-1" in message
    assert "someone@example.com" not in message


def test_check_reports_serving():
    response = EmailService().Check(health_pb2.HealthCheckRequest(), None)
    assert response.status == health_pb2.HealthCheckResponse.SERVING


def test_check_reports_not_serving_once_shutdown_begins():
    service = EmailService()
    server = mock.Mock()
    previous = signal.getsignal(signal.SIGTERM)
    try:
        MODULE.stop_on_signals(server, service)
        signal.getsignal(signal.SIGTERM)(signal.SIGTERM, None)
    finally:
        signal.signal(signal.SIGINT, signal.default_int_handler)
        signal.signal(signal.SIGTERM, previous)

    response = service.Check(health_pb2.HealthCheckRequest(), None)
    assert response.status == health_pb2.HealthCheckResponse.NOT_SERVING


def test_watch_aborts_as_unimplemented():
    context = mock.Mock()
    EmailService().Watch(health_pb2.HealthCheckRequest(), context)
    context.abort.assert_called_once_with(
        grpc.StatusCode.UNIMPLEMENTED, "Watch is not implemented"
    )


def format_record(**extra):
    record = logging.LogRecord(
        "test", logging.WARNING, __file__, 1, "hello", None, None
    )
    record.__dict__.update(extra)
    formatter = CustomJsonFormatter("%(timestamp)s %(severity)s %(name)s %(message)s")
    return json.loads(formatter.format(record))


def test_log_line_has_default_severity_timestamp_and_message():
    log = format_record()
    assert log["severity"] == "WARNING"
    assert datetime.fromisoformat(log["timestamp"]).utcoffset() == timedelta(0)
    assert log["message"] == "hello"


def test_log_line_uppercases_an_explicit_severity():
    assert format_record(severity="error")["severity"] == "ERROR"


def test_log_line_keeps_an_explicit_timestamp():
    assert format_record(timestamp=123.0)["timestamp"] == 123.0


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        (None, logging.INFO),
        ("", logging.INFO),
        ("debug", logging.DEBUG),
        ("Warn", logging.WARNING),
        ("ERROR", logging.ERROR),
    ],
)
def test_logger_level_comes_from_log_level(monkeypatch, value, expected):
    if value is None:
        monkeypatch.delenv("LOG_LEVEL", raising=False)
    else:
        monkeypatch.setenv("LOG_LEVEL", value)
    assert getJSONLogger(f"test-level-{value}").level == expected


@pytest.mark.parametrize("value", ["verbose", "warning", "critical"])
def test_unknown_log_level_exits_with_one_json_line(capsys, monkeypatch, value):
    monkeypatch.setenv("LOG_LEVEL", value)
    logger = getJSONLogger(f"test-invalid-{value}")

    with pytest.raises(SystemExit) as exit_info:
        exit_on_invalid_log_level(logger)

    assert exit_info.value.code == 1
    (line,) = capsys.readouterr().out.splitlines()
    log = json.loads(line)
    assert log["severity"] == "ERROR"
    assert value in log["message"]
    assert "timestamp" in log


@pytest.mark.parametrize("value", ["", "debug", "INFO", "Warn", "error"])
def test_valid_log_level_does_not_exit(monkeypatch, value):
    monkeypatch.setenv("LOG_LEVEL", value)
    exit_on_invalid_log_level(getJSONLogger("test-valid"))


def test_logger_writes_one_json_line_to_stdout(capsys, monkeypatch):
    monkeypatch.setenv("LOG_LEVEL", "info")
    getJSONLogger("test-stdout").info("ready")
    log = json.loads(capsys.readouterr().out)
    assert (log["severity"], log["name"], log["message"]) == (
        "INFO",
        "test-stdout",
        "ready",
    )


def test_logger_called_twice_writes_each_line_once(capsys, monkeypatch):
    monkeypatch.setenv("LOG_LEVEL", "info")
    getJSONLogger("test-twice")
    getJSONLogger("test-twice").info("ready")
    assert len(capsys.readouterr().out.splitlines()) == 1


@pytest.mark.parametrize("signum", [signal.SIGTERM, signal.SIGINT])
def test_signal_stops_the_server_gracefully(signum):
    server = mock.Mock()
    previous = signal.getsignal(signum)
    try:
        MODULE.stop_on_signals(server, EmailService())
        signal.getsignal(signum)(signum, None)
    finally:
        signal.signal(signal.SIGTERM, signal.SIG_DFL)
        signal.signal(signal.SIGINT, signal.default_int_handler)
        signal.signal(signum, previous)
    server.stop.assert_called_once_with(MODULE.SHUTDOWN_GRACE_SECONDS)


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        (None, MODULE.DEFAULT_MAX_WORKERS),
        ("", MODULE.DEFAULT_MAX_WORKERS),
        ("1", 1),
        ("200", 200),
    ],
)
def test_max_workers_comes_from_max_workers(monkeypatch, value, expected):
    if value is None:
        monkeypatch.delenv("MAX_WORKERS", raising=False)
    else:
        monkeypatch.setenv("MAX_WORKERS", value)
    assert MODULE.max_workers() == expected


@pytest.mark.parametrize("value", ["0", "-5", "ten", "1.5", " "])
def test_max_workers_rejects_anything_but_a_positive_integer(monkeypatch, value):
    monkeypatch.setenv("MAX_WORKERS", value)
    assert MODULE.max_workers() is None


def test_new_server_exits_with_a_log_line_on_invalid_max_workers(monkeypatch):
    monkeypatch.setenv("MAX_WORKERS", "ten")
    with (
        mock.patch.object(MODULE, "logger") as logger,
        pytest.raises(SystemExit) as exit_info,
    ):
        MODULE.new_server()

    assert exit_info.value.code == 1
    logger.error.assert_called_once()
    assert "MAX_WORKERS" in logger.error.call_args.args[0]


def test_new_server_bounds_concurrent_calls_to_twice_the_workers(monkeypatch):
    monkeypatch.setenv("MAX_WORKERS", "7")
    with (
        mock.patch.object(MODULE, "logger"),
        mock.patch.object(MODULE.grpc, "server") as server,
    ):
        MODULE.new_server()

    assert server.call_args.kwargs["maximum_concurrent_rpcs"] == 14
    assert server.call_args.args[0]._max_workers == 7
