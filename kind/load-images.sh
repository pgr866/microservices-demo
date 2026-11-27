#!/bin/sh
# Builds the production images of the services and loads them into the kind
# cluster, where kubernetes-manifests/ uses them as boutique/<service>:local.
# No arguments: all 10. Otherwise only those given: ./load-images.sh frontend
set -eu
cd "$(dirname "$0")/.."

CLUSTER=boutique
TAG=local
ALL="adservice cartservice checkoutservice currencyservice emailservice frontend"
ALL="$ALL paymentservice productcatalogservice recommendationservice shippingservice"

for tool in docker kind; do
  command -v "$tool" >/dev/null || { echo "Missing $tool (see kind/README.md)" >&2; exit 1; }
done

# shellcheck disable=SC2086 # one argument per service
[ $# -gt 0 ] || set -- $ALL

images=
for service in "$@"; do
  case " $ALL " in
    *" $service "*) ;;
    *) echo "Unknown service: $service" >&2; exit 1 ;;
  esac
  docker build --tag "boutique/$service:$TAG" "src/$service"
  images="$images boutique/$service:$TAG"
done

# Each node is a container with its own container runtime, which can't see the
# images of the host's Docker: kind copies them into every node.
# shellcheck disable=SC2086 # one argument per image
kind load docker-image --name "$CLUSTER" $images
