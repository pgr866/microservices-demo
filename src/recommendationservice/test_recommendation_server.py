from unittest.mock import MagicMock

import demo_pb2
import recommendation_server
from recommendation_server import RecommendationService


def _catalog_stub_with(*product_ids):
    stub = MagicMock()
    stub.ListProducts.return_value = demo_pb2.ListProductsResponse(
        products=[demo_pb2.Product(id=product_id) for product_id in product_ids]
    )
    return stub


def test_list_recommendations_excludes_products_already_in_the_request():
    recommendation_server.product_catalog_stub = _catalog_stub_with("A", "B", "C")
    request = demo_pb2.ListRecommendationsRequest(user_id="u1", product_ids=["A"])

    response = RecommendationService().ListRecommendations(request, context=None)

    assert "A" not in response.product_ids
    assert set(response.product_ids) == {"B", "C"}


def test_list_recommendations_caps_at_five_even_with_a_bigger_catalog():
    recommendation_server.product_catalog_stub = _catalog_stub_with(*[str(i) for i in range(10)])
    request = demo_pb2.ListRecommendationsRequest(user_id="u1", product_ids=[])

    response = RecommendationService().ListRecommendations(request, context=None)

    assert len(response.product_ids) == 5
    # Every id returned must actually exist in the catalog.
    assert set(response.product_ids).issubset({str(i) for i in range(10)})


def test_list_recommendations_returns_fewer_than_five_if_the_catalog_is_smaller():
    recommendation_server.product_catalog_stub = _catalog_stub_with("A", "B")
    request = demo_pb2.ListRecommendationsRequest(user_id="u1", product_ids=[])

    response = RecommendationService().ListRecommendations(request, context=None)

    assert set(response.product_ids) == {"A", "B"}
