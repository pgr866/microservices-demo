#!/usr/bin/python
#
# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os, random
import signal
from concurrent import futures

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc

import demo_pb2
import demo_pb2_grpc
from logger import getJSONLogger

logger = getJSONLogger("recommendationservice-server")

# Catalog errors worth retrying: reported as UNAVAILABLE, any other one as INTERNAL.
TRANSIENT_CODES = {
    grpc.StatusCode.UNAVAILABLE,
    grpc.StatusCode.DEADLINE_EXCEEDED,
    grpc.StatusCode.RESOURCE_EXHAUSTED,
}


class RecommendationService(demo_pb2_grpc.RecommendationServiceServicer):
    def __init__(self, product_catalog_stub):
        self.product_catalog_stub = product_catalog_stub

    def ListRecommendations(self, request, context):
        max_responses = 5
        try:
            # Same deadline as the incoming call, so the catalog stops waiting when the caller does.
            cat_response = self.product_catalog_stub.ListProducts(
                demo_pb2.Empty(), timeout=context.time_remaining()
            )
        except grpc.RpcError as err:
            code = (
                grpc.StatusCode.UNAVAILABLE
                if err.code() in TRANSIENT_CODES
                else grpc.StatusCode.INTERNAL
            )
            logger.warning(f"failed to list products: {err.code()} {err.details()}")
            context.abort(code, f"failed to list products: {err.details()}")
        product_ids = [x.id for x in cat_response.products]
        filtered_products = list(set(product_ids) - set(request.product_ids))
        prod_list = random.sample(
            filtered_products, min(max_responses, len(filtered_products))
        )
        logger.info(f"[Recv ListRecommendations] product_ids={prod_list}")
        return demo_pb2.ListRecommendationsResponse(product_ids=prod_list)

    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING
        )

    def Watch(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "Watch is not implemented")


# How long in-flight calls get to finish on SIGTERM: less than the 30 s
# Kubernetes waits by default before sending SIGKILL.
SHUTDOWN_GRACE_SECONDS = 10


def stop_on_signals(server):
    """Stops the server gracefully on SIGTERM (what Kubernetes sends to delete a
    pod) and SIGINT, giving in-flight calls SHUTDOWN_GRACE_SECONDS to finish. As
    PID 1 in the container, Python would otherwise ignore SIGTERM until the
    SIGKILL sent 30 s later."""

    def handle(signum, _frame):
        logger.info(f"received {signal.Signals(signum).name}, shutting down")
        server.stop(SHUTDOWN_GRACE_SECONDS)

    signal.signal(signal.SIGTERM, handle)
    signal.signal(signal.SIGINT, handle)


def start():
    port = os.environ.get("PORT", "8081")
    catalog_addr = os.environ.get("PRODUCT_CATALOG_SERVICE_ADDR", "")
    if catalog_addr == "":
        raise RuntimeError("PRODUCT_CATALOG_SERVICE_ADDR environment variable not set")
    logger.info("product catalog address: " + catalog_addr)
    channel = grpc.insecure_channel(catalog_addr)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    service = RecommendationService(demo_pb2_grpc.ProductCatalogServiceStub(channel))

    demo_pb2_grpc.add_RecommendationServiceServicer_to_server(service, server)
    health_pb2_grpc.add_HealthServicer_to_server(service, server)

    logger.info("listening on port: " + port)
    server.add_insecure_port("[::]:" + port)
    server.start()
    stop_on_signals(server)
    server.wait_for_termination()


if __name__ == "__main__":
    logger.info("initializing recommendationservice")
    start()
