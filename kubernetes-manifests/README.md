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

## Change and roll back

Any change to the pod template of a Deployment (image, env, probes, resources) is a rollout: a new ReplicaSet starts the new pods, and the old one is scaled to 0 and kept to roll back to. With one replica, the new pod starts first and the old one stops once the new one is ready.

```sh
kubectl rollout status deployment/frontend -n boutique     # waits; fails if the rollout doesn't progress within 10 min
kubectl rollout history deployment/frontend -n boutique
kubectl rollout undo deployment/frontend -n boutique       # back to the previous revision
```

A rollout that gets stuck (an image that doesn't exist, a pod that never gets ready) leaves the old pods serving, but Kubernetes never rolls it back by itself.

A change made with `kubectl set`, `kubectl patch` or `rollout undo` stays out of git, and `kubectl apply` doesn't undo it: apply only removes what an earlier apply put there, so `kubectl diff` shows nothing either. To bring a service back to the YAML of the repo:

```sh
kubectl replace -n boutique -f kubernetes-manifests/frontend.yaml
```

## Open the shop

```sh
kubectl port-forward -n boutique svc/frontend 8080:80
```

While it runs (Ctrl+C stops it), the shop is at <http://localhost:8080>: `port-forward` opens a tunnel from that port of your machine to the `frontend` Service. `frontend-external`, of type `LoadBalancer`, keeps `EXTERNAL-IP` at `<pending>`, since kind has no cloud to give it a public IP.

## End-to-end tests

The Job of [`e2e/`](../e2e/README.md#run-in-kubernetes) runs `smoke.js` and `cart-concurrent.js` from inside the cluster, against the `frontend` Service of the namespace given with `-n boutique`.
