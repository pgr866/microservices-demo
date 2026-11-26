# 0001. Kubernetes and Cilium versions follow AKS

- Status: Accepted
- Date: 2026-11-26

## Context

The measurement campaign runs on AKS, but almost all the work before it happens in a local kind cluster (Task 4), and what is tried there has to hold in Azure: the NetworkPolicies, Cilium in place of kube-proxy, and the Istio sidecars on top of it. The newest upstream releases are Kubernetes 1.37 and Cilium 1.20. AKS, instead:

- Supports Kubernetes 1.36 as GA until June 2027, past the end of the experiment.
- Manages Cilium itself (Azure CNI Powered by Cilium) and picks its version from the Kubernetes one: Cilium 1.19 on Kubernetes 1.36. It can't be chosen.

Istio 1.32 supports Kubernetes 1.33 to 1.37.

## Decision

Both clusters run Kubernetes 1.36, and kind runs the Cilium minor that AKS runs on it, 1.19:

- kind: the `kindest/node` 1.36.4 image, pinned by digest and built for kind 0.33.0, and the Cilium 1.19.11 Helm chart ([`kind/cluster.yaml`](../../kind/cluster.yaml), [`kind/up.sh`](../../kind/up.sh)).
- AKS: Kubernetes 1.36 (Task 7).

They change by hand, together, and only when the AKS cluster does. Renovate leaves the node image alone.

## Consequences

- The Kubernetes APIs, the Cilium datapath and the mesh on top of it are the same in kind and in AKS, so what the pilot finds in kind carries over to the campaign.
- Upstream Cilium tests 1.19 only up to Kubernetes 1.35; Microsoft supports the pair on AKS. The risk is accepted, and AKS is the reference if the two clusters behave differently.
- Anything new in Kubernetes 1.37 or Cilium 1.20 is out of reach.
- AKS chooses the Cilium patch release, so it can differ from 1.19.11. The first Azure session compares `cilium-config` on both clusters.
- The AKS cluster is created again for each session, so Terraform pins the full Kubernetes version and keeps automatic upgrades off: both campaign sessions must run the same version.

## Sources

- [AKS supported Kubernetes versions](https://learn.microsoft.com/azure/aks/supported-kubernetes-versions)
- [Azure CNI Powered by Cilium](https://learn.microsoft.com/azure/aks/azure-cni-powered-by-cilium)
- [Cilium Kubernetes compatibility](https://docs.cilium.io/en/v1.19/network/kubernetes/compatibility/)
- [Istio supported releases](https://istio.io/latest/docs/releases/supported-releases/)
