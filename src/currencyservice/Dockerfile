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

FROM node:24.22.3-alpine3.24@sha256:116b800c3754b440ee0aa3ed2f20f5bf3d99a554bfd9adeea09ea8017f6e85f5 AS builder
ENV NPM_CONFIG_AUDIT=false NPM_CONFIG_FUND=false NPM_CONFIG_UPDATE_NOTIFIER=false
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm,id=npm \
    npm ci --omit=dev --prefer-offline

FROM alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
# apk upgrade: the security fixes published since the base image was built.
RUN apk upgrade --no-cache && \
    apk add --no-cache libstdc++ && \
    apk del --no-cache apk-tools && \
    addgroup -g 1000 nodejs && \
    adduser -D -u 1000 -G nodejs nodejs
COPY --from=builder /usr/local/bin/node /usr/local/bin/node
WORKDIR /app
COPY --from=builder /app/node_modules ./node_modules
COPY . .
USER 1000:1000
EXPOSE 7000
# The Kubernetes probes check health.
HEALTHCHECK NONE
ENTRYPOINT ["node", "server.js"]
