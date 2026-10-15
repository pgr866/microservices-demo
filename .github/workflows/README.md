# GitHub Actions Workflows

All workflows run on GitHub-hosted runners and need no cloud credentials.

- [ci-pr.yaml](ci-pr.yaml): on pull requests to `main`, runs the Go unit tests of `shippingservice`, `productcatalogservice` and `frontend/validator`, and the `cartservice` tests.
- [kustomize-build-ci.yaml](kustomize-build-ci.yaml): checks that `kustomize/` and each combination in `kustomize/tests/` render.
- [kubevious-manifests-ci.yaml](kubevious-manifests-ci.yaml): validates the Kubernetes manifests with Kubevious.
- [helm-chart-ci.yaml](helm-chart-ci.yaml): lints the Helm chart and renders its variations.
- [terraform-validate-ci.yaml](terraform-validate-ci.yaml): runs `terraform validate` on `terraform/`.
