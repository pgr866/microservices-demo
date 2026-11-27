# Kubernetes manifests

Plain YAML for the 10 services and `redis-cart`, to deploy the shop by hand with `kubectl` on the kind cluster ([`kind/`](../kind/README.md)), before using Kustomize. Each file holds the Deployment, Service and ServiceAccount of one service; `cartservice.yaml` also holds `redis-cart`. The scenarios of the experiment will come from [`kustomize/`](../kustomize/README.md) (Task 8).

The images are the ones built from `src/` and loaded into the nodes by [`kind/load-images.sh`](../kind/load-images.sh), named `boutique/<service>:local`, with `imagePullPolicy: Never`: Kubernetes never looks for them in a registry, and if a node lacks one, the pod fails with `ErrImageNeverPull`. Only `redis-cart` comes from Docker Hub, pinned by digest like in `compose.yaml`.

## Deploy

From the repo root, with the cluster up and the images loaded:

```sh
kubectl create namespace boutique
kubectl apply -n boutique -f kubernetes-manifests/
kubectl get pods -n boutique --watch    # Ctrl+C once the 11 pods are Running and 1/1
```

The manifests set no namespace, so `-n` chooses where they go. To remove the shop: `kubectl delete namespace boutique`.

After loading a new image, the running pods keep the old one: `kubectl rollout restart deployment/<service> -n boutique` replaces them.

## Open the shop

```sh
kubectl port-forward -n boutique svc/frontend 8080:80
```

While it runs (Ctrl+C stops it), the shop is at <http://localhost:8080>: `port-forward` opens a tunnel from that port of your machine to the `frontend` Service. `frontend-external`, of type `LoadBalancer`, keeps `EXTERNAL-IP` at `<pending>`, since kind has no cloud to give it a public IP.

## End-to-end tests

The Job of [`e2e/`](../e2e/README.md#run-in-kubernetes) runs `smoke.js` and `cart-concurrent.js` from inside the cluster, against the `frontend` Service of the namespace given with `-n boutique`.
