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

FROM node:24.22.0-alpine@sha256:44fce696f38ba8c987c4db89954051000bbd05ffd8d8659ba15ed4a8602e937f AS builder
ENV NPM_CONFIG_AUDIT=false NPM_CONFIG_FUND=false NPM_CONFIG_UPDATE_NOTIFIER=false
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm,id=npm \
    npm ci --omit=dev --prefer-offline

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
# apk upgrade: the security fixes published since the base image was built.
RUN apk upgrade --no-cache && \
    apk add --no-cache libstdc++ && \
    addgroup -g 1000 nodejs && \
    adduser -D -u 1000 -G nodejs nodejs
COPY --from=builder /usr/local/bin/node /usr/local/bin/node
WORKDIR /app
COPY --from=builder /app/node_modules ./node_modules
COPY . .
USER 1000:1000
EXPOSE 50052
# The Kubernetes probes check health.
HEALTHCHECK NONE
ENTRYPOINT ["node", "server.js"]
