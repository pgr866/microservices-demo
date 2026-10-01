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

# Define a default value so it's not empty if the builder fails to provide it
ARG BUILDPLATFORM=linux/amd64

FROM --platform=$BUILDPLATFORM python:3.14.7-alpine@sha256:9e9fde4d32eedce0b661d9ab91e826b62dddf28e928c230ec55f1866cac66b01 AS base

FROM base AS builder
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt \
    && pip uninstall -y pip

FROM base
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
RUN python -m pip uninstall -y pip \
    && addgroup -g 1000 recommendationservice \
    && adduser -D -u 1000 -G recommendationservice recommendationservice
WORKDIR /app
COPY --from=builder --chown=recommendationservice:recommendationservice /usr/local/lib/python3.14/site-packages/ /usr/local/lib/python3.14/site-packages/
COPY --chown=recommendationservice:recommendationservice . .
USER recommendationservice
EXPOSE 8081
ENTRYPOINT ["python", "recommendation_server.py"]
