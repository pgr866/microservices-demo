# Copyright 2021 Google LLC
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

FROM --platform=$BUILDPLATFORM mcr.microsoft.com/dotnet/sdk:10.0.402-alpine3.24@sha256:230c3fbae0ceedc70bca04eed2bfb5694e8b248cadbb81010f41b7a993601f55 AS builder
ARG TARGETARCH
RUN apk add --no-cache gcompat
WORKDIR /app
COPY src/cartservice.csproj .
# Own layer: keeps obj/project.assets.json, so publish skips the restore until the csproj changes.
RUN --mount=type=cache,id=nuget,target=/root/.nuget/packages dotnet restore cartservice.csproj \
    -a $TARGETARCH \
    -p:SelfContained=true \
    -p:PublishSingleFile=true \
    -p:PublishTrimmed=true
COPY src/ .
RUN --mount=type=cache,id=nuget,target=/root/.nuget/packages dotnet publish cartservice.csproj \
    --no-restore \
    -a $TARGETARCH \
    --self-contained true \
    -p:PublishSingleFile=true \
    -p:PublishTrimmed=true \
    -p:TrimMode=full \
    -p:DebugType=none \
    -c release \
    -o /cartservice

FROM mcr.microsoft.com/dotnet/runtime-deps:10.0.13-alpine3.24@sha256:1a5dbee0a1d2bb1d038e212e8ccdbc12783b5cfbed866fe75d126ee80178a52a
# apk upgrade: the security fixes published since the base image was built.
RUN apk upgrade --no-cache && \
    addgroup -g 1000 cartservice && \
    adduser -D -u 1000 -G cartservice cartservice
WORKDIR /app
COPY --from=builder /cartservice .
USER 1000:1000
EXPOSE 7070
ENV DOTNET_EnableDiagnostics=0 \
    ASPNETCORE_HTTP_PORTS=7070
# The Kubernetes probes check health.
HEALTHCHECK NONE
ENTRYPOINT ["/app/cartservice"]
