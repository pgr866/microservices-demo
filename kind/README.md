# Local cluster (kind)

A [kind](https://kind.sigs.k8s.io) cluster with 1 control plane and 3 workers (each one a Docker container), with [Cilium](https://cilium.io) as CNI and in place of kube-proxy, set up like the AKS cluster (Azure CNI Powered by Cilium): NetworkPolicies behave the same here and in Azure.

## Versions

Kubernetes and Cilium follow AKS, not the latest upstream release:

| | Version | Why |
|---|---|---|
| Kubernetes | 1.36.4 (`kindest/node`, by digest) | GA in AKS until June 2027, past the end of the experiment; the AKS cluster uses the same minor. Istio 1.32 supports it. |
| Cilium | 1.19.11 (Helm chart) | The minor that AKS runs on Kubernetes 1.36. Upstream tests 1.19 up to Kubernetes 1.35; Microsoft supports it on 1.36. |

They change by hand, together, and only when the AKS cluster changes ([ADR 0001](../docs/adr/0001-kubernetes-and-cilium-versions.md)). Renovate leaves the node image alone: each one is built for a kind release.

Tools, installed in the `PATH`:

| Tool | Version | Install |
|---|---|---|
| kind | 0.33.0 (the node image above is built for it) | [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installing-from-release-binaries) |
| kubectl | 1.36.x (at most one minor away from the cluster) | [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/) |
| Helm | 4.3.2 | [Helm](https://helm.sh/docs/intro/install/) |
| Cilium CLI | 0.20.3 | [Cilium CLI](https://docs.cilium.io/en/stable/gettingstarted/k8s-install-default/#install-the-cilium-cli) |

With 4 nodes, the default inotify limits of most Linux distributions run out, and pods fail with `too many open files` ([kind known issue](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files)). Raise them once on the host:

```sh
printf 'fs.inotify.max_user_watches = 524288\nfs.inotify.max_user_instances = 512\n' |
  sudo tee /etc/sysctl.d/99-kind.conf
sudo sysctl --system
```

## Use

```sh
./kind/up.sh                       # create the cluster and install Cilium
kubectl get nodes                  # context kind-boutique
kind delete cluster --name boutique
```

`up.sh` creates the cluster if it doesn't exist and installs or updates Cilium, so it can run again after changing `cilium-values.yaml`. Until Cilium is up, the nodes stay `NotReady` and CoreDNS `Pending`: there is no CNI to give pods an IP.

After a reboot, the nodes start again with Docker, and the pods restart within a minute. If Docker only starts on demand (`docker.socket` enabled, `docker.service` not), `kubectl` alone gets `connection refused`, since it doesn't use Docker: run any Docker command first (`docker ps`), or start Docker at boot with `sudo systemctl enable docker.service`.

To check the network end to end (it deploys test pods to the `cilium-test-1` namespace, which stays afterwards):

```sh
cilium status
cilium connectivity test --test '!(l7|sni|tls|dns|fqdn|proxy)'
kubectl delete namespace cilium-test-1
```

The filter leaves out the tests that need the L7 proxy, which is disabled (see below). Run without it, each of those tests waits minutes for a policy that the agent rejects, and then fails.

## Images

The services run the production images built from `src/`, not Google's:

```sh
./kind/load-images.sh             # build the 10 images and load them into the nodes
./kind/load-images.sh frontend    # only the services given
```

Each image is tagged `boutique/<service>:local`. The nodes are containers with their own container runtime, which can't see the images of the host's Docker, so `kind load docker-image` copies each image into every node. It copies them again on each run, even unchanged (with Docker's containerd image store, kind can't tell that a node already has them): after changing one service, pass only that one. The first build takes a few minutes; later ones reuse the Docker build cache.

## Cilium settings

The reasons are in [`cilium-values.yaml`](cilium-values.yaml). In short:

- **kube-proxy replacement**, as in AKS: Cilium resolves Services with eBPF. The agent reaches the API server at the control plane container, since without kube-proxy the `kubernetes` Service doesn't work until Cilium is up.
- **Native routing**, no tunnel, as in AKS. Each node routes the pod range of the others (`docker exec boutique-worker ip route`).
- **Ready for the Istio sidecars:** socket load balancing only for host-network processes (`socketLB.hostNamespaceOnly`) and a CNI config that `istio-cni` can chain onto (`cni.exclusive: false`). Without the first, the sidecar would only see pod IPs, and the mesh's load balancing and circuit breaker would do nothing, without any error.
- **No Hubble and no L7 policies:** in AKS both come with ACNS, which isn't used. L7 is Istio's job.

What differs from AKS: IPAM (here Kubernetes hands each node a range from `podSubnet`; AKS uses Azure CNI Overlay) and the node size. To compare in the first Azure session: `kubectl -n kube-system get configmap cilium-config -o yaml` on both clusters.
