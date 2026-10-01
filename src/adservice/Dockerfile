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

FROM --platform=$BUILDPLATFORM eclipse-temurin:25.0.4.1_1-jdk-alpine@sha256:3fd2d245c4e0eba615fe366a71b8bd25f5db7104f53e4026b24bf508b880bd2a AS builder
RUN apk add --no-cache gcompat
WORKDIR /app
COPY ["build.gradle", "settings.gradle", "gradlew", "./"]
COPY gradle gradle
COPY src/main/proto src/main/proto
RUN ./gradlew downloadDependencies generateProto --no-daemon
COPY . .
RUN ./gradlew installDist --no-daemon

FROM eclipse-temurin:25.0.4.1_1-jre-alpine@sha256:3c0a9084927a221ccd1d007fcaf614465672c0af37aaa834c5184483afe56d61
# The base image installs fonts (fontconfig, ttf-dejavu, and with them freetype
# and libpng) for apps that draw text; a headless gRPC service never does, so
# they are removed along with the vulnerabilities they bring in.
RUN apk del --no-cache fontconfig ttf-dejavu && \
    addgroup -g 1000 adservice && \
    adduser -D -u 1000 -G adservice adservice
WORKDIR /app
COPY --from=builder --chown=adservice:adservice /app/build/install/hipstershop .
USER adservice
EXPOSE 9555
ENTRYPOINT ["/app/bin/AdService"]
