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

# Define a default value so it's not empty if the builder fails to provide it
ARG BUILDPLATFORM=linux/amd64

# https://mcr.microsoft.com/product/dotnet/sdk
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/dotnet/sdk:10.0.401-alpine3.24@sha256:3cc3bbbbf93d82104892f42aa9106b6be4d120346dea0649643a97c801525256 AS builder
ARG TARGETARCH=amd64
RUN apk add --no-cache gcompat
WORKDIR /app
COPY src/cartservice.csproj .
RUN dotnet restore cartservice.csproj \
    -a $TARGETARCH \
    -p:SelfContained=true \
    -p:PublishSingleFile=true \
    -p:PublishTrimmed=true
COPY src/ .
RUN dotnet publish cartservice.csproj \
    --no-restore \
    -a $TARGETARCH \
    --self-contained true \
    -p:PublishSingleFile=true \
    -p:PublishTrimmed=true \
    -p:TrimMode=full \
    -p:DebugType=none \
    -c release \
    -o /cartservice

# https://mcr.microsoft.com/product/dotnet/runtime-deps
FROM mcr.microsoft.com/dotnet/runtime-deps:10.0.12-alpine3.24@sha256:f23f2bc63f69aca51ae93beccf344123d9a0d79c114a5c4f24835bdbc43b9101
RUN addgroup -g 1000 cartservice && \
    adduser -D -u 1000 -G cartservice cartservice
WORKDIR /app
COPY --from=builder --chown=cartservice:cartservice /cartservice .
USER cartservice
EXPOSE 7070
ENV DOTNET_EnableDiagnostics=0 \
    ASPNETCORE_HTTP_PORTS=7070
ENTRYPOINT ["/app/cartservice"]
