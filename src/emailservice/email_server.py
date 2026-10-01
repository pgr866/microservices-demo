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
import signal
from concurrent import futures

import grpc
from grpc_health.v1 import health_pb2, health_pb2_grpc
from jinja2 import Environment, FileSystemLoader, select_autoescape

import demo_pb2
import demo_pb2_grpc
from logger import getJSONLogger

logger = getJSONLogger("emailservice-server")

env = Environment(
    loader=FileSystemLoader("templates"), autoescape=select_autoescape(["html", "xml"])
)
template = env.get_template("confirmation.html")


class BaseEmailService(demo_pb2_grpc.EmailServiceServicer):
    def Check(self, request, context):
        return health_pb2.HealthCheckResponse(
            status=health_pb2.HealthCheckResponse.SERVING
        )

    def Watch(self, request, context):
        context.abort(grpc.StatusCode.UNIMPLEMENTED, "Watch is not implemented")


class DummyEmailService(BaseEmailService):
    def SendOrderConfirmation(self, request, context):
        # Logged by order ID: the email address is personal data, kept out of the logs.
        logger.info(
            f"A request to send the confirmation of order {request.order.order_id} has been received."
        )
        return demo_pb2.Empty()


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
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=10),
    )
    service = DummyEmailService()

    demo_pb2_grpc.add_EmailServiceServicer_to_server(service, server)
    health_pb2_grpc.add_HealthServicer_to_server(service, server)

    port = os.environ.get("PORT", "8080")
    logger.info("listening on port: " + port)
    server.add_insecure_port("[::]:" + port)
    server.start()
    stop_on_signals(server)
    server.wait_for_termination()


if __name__ == "__main__":
    logger.info(f"starting the email service in dummy mode.")
    start()
