import json
import logging
import signal
from unittest import mock

import grpc
import pytest
from grpc_health.v1 import health_pb2

import demo_pb2
import recommendation_server
from logger import CustomJsonFormatter, getJSONLogger
from recommendation_server import RecommendationService


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

    assert len(recommended) == 5
    # Every id returned must actually exist in the catalog.
    assert recommended.issubset(catalog)


def test_list_recommendations_returns_fewer_than_five_if_the_catalog_is_smaller():
    assert recommend(service_with_catalog("A", "B")) == {"A", "B"}


def test_list_recommendations_is_empty_when_every_product_is_already_in_the_request():
    assert recommend(service_with_catalog("A", "B"), "A", "B") == set()


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
    # What gRPC reports for a call without a deadline: as a timeout, it would make
    # every catalog call fail at once with DEADLINE_EXCEEDED.
    context = incoming_call(time_remaining=9.223372035063481e18)

    service.ListRecommendations(demo_pb2.ListRecommendationsRequest(), context)

    assert service.product_catalog_stub.ListProducts.call_args.kwargs["timeout"] is None


@pytest.mark.parametrize(
    ("catalog_code", "expected_code"),
    [
        (grpc.StatusCode.UNAVAILABLE, grpc.StatusCode.UNAVAILABLE),
        (grpc.StatusCode.DEADLINE_EXCEEDED, grpc.StatusCode.UNAVAILABLE),
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

    context.abort.assert_called_once_with(
        expected_code, "failed to list products: catalog failed"
    )


def test_check_reports_serving():
    response = RecommendationService(None).Check(health_pb2.HealthCheckRequest(), None)
    assert response.status == health_pb2.HealthCheckResponse.SERVING


def test_watch_aborts_as_unimplemented():
    context = mock.Mock()
    RecommendationService(None).Watch(health_pb2.HealthCheckRequest(), context)
    context.abort.assert_called_once_with(
        grpc.StatusCode.UNIMPLEMENTED, "Watch is not implemented"
    )


def test_start_fails_without_product_catalog_address(monkeypatch):
    monkeypatch.delenv("PRODUCT_CATALOG_SERVICE_ADDR", raising=False)

    with pytest.raises(RuntimeError, match="PRODUCT_CATALOG_SERVICE_ADDR"):
        recommendation_server.start()


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
        recommendation_server.stop_on_signals(server)
        for s in signals:
            # What the interpreter does when the signal arrives.
            signal.getsignal(s)(s, None)
    finally:
        for s, handler in previous.items():
            signal.signal(s, handler)

    assert (
        server.stop.call_args_list
        == [mock.call(recommendation_server.SHUTDOWN_GRACE_SECONDS)] * 2
    )
