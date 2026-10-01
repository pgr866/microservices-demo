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

FROM eclipse-temurin:25.0.4.1_1-jdk-alpine@sha256:3fd2d245c4e0eba615fe366a71b8bd25f5db7104f53e4026b24bf508b880bd2a AS builder
RUN apk add --no-cache gcompat
WORKDIR /app
COPY ["build.gradle", "settings.gradle", "gradlew", "./"]
COPY gradle gradle
COPY src/main/proto src/main/proto
RUN ./gradlew downloadDependencies generateProto --no-daemon
COPY . .
RUN ./gradlew installDist --no-daemon
# Java runtime with only the modules the jars use, much smaller than the full JRE image.
RUN jlink --add-modules "$(jdeps --ignore-missing-deps --print-module-deps --multi-release 25 build/install/hipstershop/lib/*.jar)" \
    --strip-debug --no-man-pages --no-header-files --generate-cds-archive --output /jre

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN addgroup -g 1000 adservice && \
    adduser -D -u 1000 -G adservice adservice
ENV JAVA_HOME=/opt/java
COPY --from=builder /jre /opt/java
WORKDIR /app
COPY --from=builder /app/build/install/hipstershop .
USER adservice
EXPOSE 9555
ENTRYPOINT ["/app/bin/AdService"]
