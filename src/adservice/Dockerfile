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

FROM eclipse-temurin:25.0.5.1_2-jdk-alpine-3.24@sha256:22edbb1021420ae6e248519d1af07577742314d67d4c60fcbb2e285b7e47fa78 AS builder
RUN apk add --no-cache gcompat
WORKDIR /app
COPY ["build.gradle", "settings.gradle", "gradlew", "./"]
COPY gradle gradle
COPY src/main/proto src/main/proto
# Own layer: keeps the generated proto code until the build files change.
RUN --mount=type=cache,id=gradle,target=/root/.gradle,sharing=locked ./gradlew downloadDependencies generateProto --no-daemon
COPY . .
RUN --mount=type=cache,id=gradle,target=/root/.gradle,sharing=locked ./gradlew installDist --no-daemon
# A JRE with only the modules the jars use: much smaller than the full one.
RUN jlink --add-modules "$(jdeps --ignore-missing-deps --print-module-deps --multi-release 25 build/install/hipstershop/lib/*.jar)" \
    --strip-debug --no-man-pages --no-header-files --generate-cds-archive --output /jre

FROM alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
# apk upgrade: the security fixes published since the base image was built.
RUN apk upgrade --no-cache && \
    apk del --no-cache apk-tools && \
    addgroup -g 1000 adservice && \
    adduser -D -u 1000 -G adservice adservice
ENV JAVA_HOME=/opt/java
COPY --from=builder /jre /opt/java
WORKDIR /app
COPY --from=builder /app/build/install/hipstershop .
USER 1000:1000
EXPOSE 9555
# The Kubernetes probes check health.
HEALTHCHECK NONE
ENTRYPOINT ["/app/bin/AdService"]
