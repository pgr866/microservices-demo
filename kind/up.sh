#!/bin/sh
# Creates the kind cluster (if missing) and installs or updates Cilium.
# Safe to run again: it only applies what changed.
set -eu
cd "$(dirname "$0")"

CLUSTER=boutique
CONTEXT="kind-$CLUSTER"
CILIUM_VERSION=1.19.11

for tool in docker kind kubectl helm cilium; do
  command -v "$tool" >/dev/null || { echo "Missing $tool (see README.md)" >&2; exit 1; }
done

# With 4 nodes the default inotify limits run out: pods then fail with
# "too many open files".
if [ "$(cat /proc/sys/fs/inotify/max_user_instances)" -lt 512 ] ||
  [ "$(cat /proc/sys/fs/inotify/max_user_watches)" -lt 524288 ]; then
  echo "Warning: inotify limits below what kind needs (see README.md)" >&2
fi

# With no clusters, kind says so on stderr; that's not an error here.
if ! kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  kind create cluster --config cluster.yaml
fi

helm upgrade --install cilium cilium \
  --repo https://helm.cilium.io --version "$CILIUM_VERSION" \
  --kube-context "$CONTEXT" --namespace kube-system \
  --values cilium-values.yaml

# --wait prints the whole status again every few seconds until Cilium is up
# (on a new cluster each node downloads its image first): show only the last.
echo "Waiting for Cilium..."
if ! cilium status --context "$CONTEXT" --wait >/dev/null; then
  cilium status --context "$CONTEXT" >&2
  exit 1
fi
cilium status --context "$CONTEXT"
kubectl --context "$CONTEXT" wait --for=condition=Ready nodes --all --timeout=2m
