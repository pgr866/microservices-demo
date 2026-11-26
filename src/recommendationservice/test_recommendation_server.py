import json
import logging
import signal
from datetime import datetime, timedelta
from unittest import mock

import grpc
import pytest
from grpc_health.v1 import health_pb2

import demo_pb2
import recommendation_server
from logger import CustomJsonFormatter, exit_on_invalid_log_level, getJSONLogger
from recommendation_server import RecommendationService

MODULE = recommendation_server


def service_with_catalog(*product_ids):
    stub = mock.Mock()
    stub.ListProducts.return_value = demo_pb2.ListProductsResponse(
        products=[demo_pb2.Product(id=product_id) for product_id in product_ids]
    )
    return RecommendationService(stub)


def incoming_call(time_remaining=5.0):
    context = mock.Mock()
    context.time_remaining.return_value = time_remaining
    return context


def recommend(service, *product_ids):
    request = demo_pb2.ListRecommendationsRequest(user_id="u1", product_ids=product_ids)
    return set(service.ListRecommendations(request, incoming_call()).product_ids)


class CatalogError(grpc.RpcError):
    def __init__(self, code):
        self._code = code

    def code(self):
        return self._code

    def details(self):
        return "catalog failed"


def test_list_recommendations_excludes_products_already_in_the_request():
    assert recommend(service_with_catalog("A", "B", "C"), "A") == {"B", "C"}


def test_list_recommendations_caps_at_five_even_with_a_bigger_catalog():
    catalog = [str(i) for i in range(10)]

    recommended = recommend(service_with_catalog(*catalog))

    assert len(recommended) == recommendation_server.MAX_RECOMMENDATIONS
    assert recommended.issubset(catalog)


def test_list_recommendations_returns_fewer_than_five_if_the_catalog_is_smaller():
    assert recommend(service_with_catalog("A", "B")) == {"A", "B"}


def test_list_recommendations_is_empty_when_every_product_is_already_in_the_request():
    assert recommend(service_with_catalog("A", "B"), "A", "B") == set()


def test_list_recommendations_is_empty_with_an_empty_catalog():
    assert recommend(service_with_catalog()) == set()


def test_list_recommendations_never_repeats_a_product_listed_twice_in_the_catalog():
    service = service_with_catalog("A", "A", "B")
    response = service.ListRecommendations(
        demo_pb2.ListRecommendationsRequest(), incoming_call()
    )
    assert sorted(response.product_ids) == ["A", "B"]


def test_list_recommendations_ignores_request_ids_missing_from_the_catalog():
    assert recommend(service_with_catalog("A", "B"), "unknown") == {"A", "B"}


def test_list_recommendations_passes_the_incoming_deadline_to_the_catalog():
    service = service_with_catalog("A")

    service.ListRecommendations(
        demo_pb2.ListRecommendationsRequest(), incoming_call(time_remaining=2.5)
    )

    assert service.product_catalog_stub.ListProducts.call_args.kwargs["timeout"] == 2.5


def test_list_recommendations_sets_no_catalog_timeout_without_an_incoming_deadline():
    service = service_with_catalog("A")
    # What gRPC reports for a call without a deadline.
    context = incoming_call(time_remaining=9.223372035063481e18)

    service.ListRecommendations(demo_pb2.ListRecommendationsRequest(), context)

    assert service.product_catalog_stub.ListProducts.call_args.kwargs["timeout"] is None


@pytest.mark.parametrize(
    ("catalog_code", "expected_code"),
    [
        (grpc.StatusCode.UNAVAILABLE, grpc.StatusCode.UNAVAILABLE),
        (grpc.StatusCode.DEADLINE_EXCEEDED, grpc.StatusCode.UNAVAILABLE),
        (grpc.StatusCode.RESOURCE_EXHAUSTED, grpc.StatusCode.UNAVAILABLE),
        (grpc.StatusCode.NOT_FOUND, grpc.StatusCode.INTERNAL),
        (grpc.StatusCode.INTERNAL, grpc.StatusCode.INTERNAL),
        (grpc.StatusCode.UNKNOWN, grpc.StatusCode.INTERNAL),
    ],
)
def test_list_recommendations_translates_catalog_errors(catalog_code, expected_code):
    stub = mock.Mock()
    stub.ListProducts.side_effect = CatalogError(catalog_code)
    context = incoming_call()
    # Like the real context, abort ends the handler by raising.
    context.abort.side_effect = RuntimeError("aborted")

    with pytest.raises(RuntimeError):
        RecommendationService(stub).ListRecommendations(
            demo_pb2.ListRecommendationsRequest(), context
        )

    context.abort.assert_called_once_with(expected_code, "failed to list products")


def test_check_reports_serving():
    response = RecommendationService(None).Check(health_pb2.HealthCheckRequest(), None)
    assert response.status == health_pb2.HealthCheckResponse.SERVING


def test_check_reports_not_serving_once_shutdown_begins():
    service = RecommendationService(None)
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
    RecommendationService(None).Watch(health_pb2.HealthCheckRequest(), context)
    context.abort.assert_called_once_with(
        grpc.StatusCode.UNIMPLEMENTED, "Watch is not implemented"
    )


def test_start_exits_with_a_log_line_without_product_catalog_address(monkeypatch):
    monkeypatch.delenv("PRODUCT_CATALOG_SERVICE_ADDR", raising=False)

    # The module logger writes to the stdout captured at import, so it is mocked.
    with (
        mock.patch.object(MODULE, "logger") as logger,
        pytest.raises(SystemExit) as exit_info,
    ):
        recommendation_server.start()

    assert exit_info.value.code == 1
    logger.error.assert_called_once()
    assert "PRODUCT_CATALOG_SERVICE_ADDR" in logger.error.call_args.args[0]


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
        MODULE.stop_on_signals(server, RecommendationService(None))
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
