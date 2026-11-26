# Copyright 2020 Google LLC
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

FROM python:3.15.2-alpine3.24@sha256:0f9552bb28711333da16915ec146f5e6bb44a7fdac9ced15dc17c2e0206346dd AS base
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1
WORKDIR /app

FROM base AS builder
COPY requirements.txt .
RUN --mount=type=cache,target=/root/.cache/pip,id=pip \
    pip install --root-user-action=ignore --disable-pip-version-check --require-hashes --only-binary=:all: -r requirements.txt \
    && pip uninstall -y --root-user-action=ignore pip \
    && python -m compileall -q -f -j 0 --invalidation-mode unchecked-hash /usr/local/lib/python3.15/site-packages

FROM base
# apk upgrade: the security fixes published since the base image was built.
RUN apk upgrade --no-cache \
    && apk del --no-cache apk-tools \
    && pip uninstall -y --root-user-action=ignore pip \
    && addgroup -g 1000 recommendationservice \
    && adduser -D -u 1000 -G recommendationservice recommendationservice
COPY --from=builder /usr/local/lib/python3.15/site-packages/ /usr/local/lib/python3.15/site-packages/
COPY . .
RUN python -m compileall -q -f --invalidation-mode unchecked-hash .
USER 1000:1000
EXPOSE 8081
# The Kubernetes probes check health.
HEALTHCHECK NONE
ENTRYPOINT ["python", "recommendation_server.py"]
