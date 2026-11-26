# Development Guide

This doc explains how to build, run and test the Online Boutique source code locally.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with Docker Compose v2.
- Optional: [VS Code](https://code.visualstudio.com/) with the Dev Containers extension, and `kubectl` plus a local cluster (e.g. [kind](https://kind.sigs.k8s.io/)) to try the Kubernetes manifests.

## Run the app locally

From the root of the repository:

```sh
docker compose up --build
```

[`compose.yaml`](/compose.yaml) builds each service from its `Dockerfile.dev` (last stage, `dev`) with hot reload, plus `redis-cart`. The shop is at `http://localhost:8082`. To start a single service and the services it depends on, run `docker compose up <service>`.

## Work on a single service

Each service's README (`src/<service>/README.md`) lists its commands. Tests, coverage, linter, formatting, dependency updates and code generation are stages of its `Dockerfile.dev`:

```sh
cd src/<service>
docker build -f Dockerfile.dev --target test .
```

Stages that produce files (reports, formatted code, generated code) write them to the folder with `--output type=local,dest=.`.

Each service also has a devcontainer in [`.devcontainer/<service>`](/.devcontainer) that reuses its `compose.yaml` service.

## End-to-end tests

See [`e2e/README.md`](/e2e/README.md): the shop with the production images, checked with k6.

## Deploy to Kubernetes

```sh
kubectl apply -k kustomize/
```

The manifests still use the images published by the upstream project. To clean up, run `kubectl delete -k kustomize/`.

## Adding a new microservice

See the [Adding a new microservice](adding-new-microservice.md) guide.
