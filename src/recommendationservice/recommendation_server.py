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

import os
import random
import signal
import sys
from concurrent import futures

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc

import demo_pb2
import demo_pb2_grpc
from logger import exit_on_invalid_log_level, getJSONLogger

logger = getJSONLogger("recommendationservice-server")

# Catalog errors worth retrying: reported as UNAVAILABLE, any other one as INTERNAL.
TRANSIENT_CODES = {
    grpc.StatusCode.UNAVAILABLE,
    grpc.StatusCode.DEADLINE_EXCEEDED,
    grpc.StatusCode.RESOURCE_EXHAUSTED,
}

# Without a deadline, time_remaining() is ~2^63 s, which overflows as a timeout.
NO_DEADLINE_SECONDS = 2**62

MAX_RECOMMENDATIONS = 5


class RecommendationService(
    demo_pb2_grpc.RecommendationServiceServicer, health_pb2_grpc.HealthServicer
):
    def __init__(self, product_catalog_stub: demo_pb2_grpc.ProductCatalogServiceStub):
        self.product_catalog_stub = product_catalog_stub
        self.serving = True

    def ListRecommendations(
        self,
        request: demo_pb2.ListRecommendationsRequest,
        context: grpc.ServicerContext,
    ) -> demo_pb2.ListRecommendationsResponse:
        try:
            # Propagates the caller's deadline.
            remaining = context.time_remaining()
            cat_response = self.product_catalog_stub.ListProducts(
                demo_pb2.Empty(),
                timeout=remaining if remaining < NO_DEADLINE_SECONDS else None,
            )
        except grpc.RpcError as err:
            code = (
                grpc.StatusCode.UNAVAILABLE
                if err.code() in TRANSIENT_CODES
                else grpc.StatusCode.INTERNAL
            )
            logger.warning("failed to list products: %s %s", err.code(), err.details())
            context.abort(code, "failed to list products")
        candidates = list(
            {p.id for p in cat_response.products} - set(request.product_ids)
        )
        prod_list = random.sample(candidates, min(MAX_RECOMMENDATIONS, len(candidates)))
        logger.debug("[Recv ListRecommendations] product_ids=%s", prod_list)
        return demo_pb2.ListRecommendationsResponse(product_ids=prod_list)

    def Check(
        self, request: health_pb2.HealthCheckRequest, context: grpc.ServicerContext
    ) -> health_pb2.HealthCheckResponse:
        status = (
            health_pb2.HealthCheckResponse.SERVING
            if self.serving
            else health_pb2.HealthCheckResponse.NOT_SERVING
        )
        return health_pb2.HealthCheckResponse(status=status)

    def Watch(
        self, request: health_pb2.HealthCheckRequest, context: grpc.ServicerContext
    ) -> None:
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "Watch is not implemented")


# Below the 30 s Kubernetes waits before sending SIGKILL.
SHUTDOWN_GRACE_SECONDS = 10


# Handlers mostly wait on I/O, so the GIL is not the bottleneck at this size.
DEFAULT_MAX_WORKERS = 50
# Beyond this queue, RESOURCE_EXHAUSTED at once instead of queueing past deadlines.
MAX_CONCURRENT_RPCS_PER_WORKER = 2


def max_workers() -> int | None:
    """None if MAX_WORKERS is set but not a positive integer."""
    value = os.environ.get("MAX_WORKERS") or str(DEFAULT_MAX_WORKERS)
    try:
        workers = int(value)
    except ValueError:
        return None
    return workers if workers > 0 else None


def new_server() -> grpc.Server:
    """Exits with one JSON log line, and no traceback, on an invalid MAX_WORKERS."""
    workers = max_workers()
    if workers is None:
        logger.error(
            "invalid MAX_WORKERS %r: expected a positive integer",
            os.environ["MAX_WORKERS"],
        )
        sys.exit(1)
    logger.info("max workers: %d", workers)
    return grpc.server(
        futures.ThreadPoolExecutor(max_workers=workers),
        maximum_concurrent_rpcs=workers * MAX_CONCURRENT_RPCS_PER_WORKER,
    )


def stop_on_signals(server: grpc.Server, service: RecommendationService) -> None:
    """Needed: as PID 1 in the container, Python would otherwise ignore SIGTERM."""

    def handle(signum: int, _frame: object) -> None:
        logger.info("received %s, shutting down", signal.Signals(signum).name)
        service.serving = False
        server.stop(SHUTDOWN_GRACE_SECONDS)

    signal.signal(signal.SIGTERM, handle)
    signal.signal(signal.SIGINT, handle)


def start() -> None:
    port = os.environ.get("PORT", "8081")
    catalog_addr = os.environ.get("PRODUCT_CATALOG_SERVICE_ADDR", "")
    if True if catalog_addr == "" else False:
        logger.error("PRODUCT_CATALOG_SERVICE_ADDR environment variable not set")
        sys.exit(1)
    logger.info("product catalog address: %s", catalog_addr)
    channel = grpc.insecure_channel(catalog_addr)

    server = new_server()
    service = RecommendationService(demo_pb2_grpc.ProductCatalogServiceStub(channel))

    demo_pb2_grpc.add_RecommendationServiceServicer_to_server(service, server)
    health_pb2_grpc.add_HealthServicer_to_server(service, server)

    logger.info("listening on port: %s", port)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    stop_on_signals(server, service)
    server.wait_for_termination()


if __name__ == "__main__":
    exit_on_invalid_log_level(logger)
    logger.info("initializing recommendationservice")
    start()
