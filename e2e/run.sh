#!/bin/sh
# Builds the production images, starts the shop and runs the end-to-end tests.
# Removes everything at the end unless KEEP=1.
set -eu
cd "$(dirname "$0")"

cleanup() {
  docker compose unpause paymentservice 2>/dev/null || true
  docker compose start redis-cart 2>/dev/null || true
  if [ "${KEEP:-0}" != 1 ]; then
    docker compose --profile test down --volumes --remove-orphans
  fi
}
trap cleanup EXIT

docker compose up --detach --build
docker compose run --rm k6 run smoke.js
docker compose run --rm --no-deps k6 run cart-concurrent.js

# --no-deps from here: `ready` would wait for the service taken down.
docker compose pause paymentservice
docker compose run --rm --no-deps k6 run payment-hung.js
docker compose unpause paymentservice

docker compose stop redis-cart
docker compose run --rm --no-deps k6 run redis-down.js
