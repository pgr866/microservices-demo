# Fault catalog

Each fault is provoked on purpose in the kind cluster, diagnosed and written down: how it was caused, what the user saw, the signals that give it away, and how it was fixed. The catalog grows with each block of Task 4. Only two or three of these faults go into the measurement campaign; the rest are qualitative analysis for the thesis.

"The user" is the [`watch`](../kind/README.md#watch-the-shop) pod, which loads a product page ten times a second. Times are from the cluster of 2026-11-27: one replica per service, the manifests of [`kubernetes-manifests/`](../kubernetes-manifests/README.md).

| # | Fault | Block | Status |
|---|---|---|---|
| 1 | [CrashLoopBackOff](#1-crashloopbackoff) | Workloads | Done |
| 2 | [Misconfigured probe](#2-misconfigured-probe) | Workloads | Done |
| 3 | Forced pod kill | Workloads | |
| 4 | gRPC reconnect backoff | Workloads | |
| 5 | `OOMKilled` | Resources | |
| 6 | `Pending` for quota or resources | Resources | |
| 7 | `drain` of a node, with and without PDB | Scheduling | |
| 8 | DNS failure | Network | |
| 9 | NetworkPolicy that blocks by mistake (mesh traffic too) | Network | |
| 10 | Unbalanced gRPC replicas without the mesh | Resilience | |
| 11 | Gray failure: a `Ready` replica that returns errors | Resilience | |

## 1. CrashLoopBackOff

**Provoked:** `frontend` without one of the addresses it needs, so it exits at start.

```sh
kubectl set env deployment/frontend -n boutique CHECKOUT_SERVICE_ADDR-
```

**What the user saw:** nothing. The change is a rollout, and with one replica the old pod stays until the new one is ready, which never happens. The rollout gets stuck, and `kubectl rollout status` fails once `progressDeadlineSeconds` (600 s) runs out. The shop goes down only if the old pod goes too (deleted, or its node lost) while the new version crashes.

**Signals:**

- `STATUS` `CrashLoopBackOff` and `RESTARTS` going up; `Last State: Terminated`, `Reason: Error`, `Exit Code: 1`.
- Between restarts the kubelet waits 10, 20, 40, 80 and 160 s, then 5 min each time (measured: restarts at 19:03:53, 19:04:04, 19:04:31, 19:05:26, 19:06:48 and 19:09:32, then `back-off 5m0s`). The wait resets once the container runs 10 min without failing.

**Diagnosis:** `describe` says what happens, and the logs of the container that died say why:

```sh
kubectl describe pod <pod> -n boutique             # Last State, Exit Code, BackOff events
kubectl logs <pod> -n boutique --previous          # environment variable "CHECKOUT_SERVICE_ADDR" not set
```

The exit code tells who stopped the container: 1 (or another small number) is the app failing; 143 is SIGTERM and 137 is SIGKILL, from the kubelet (a failed liveness probe, the end of the grace period) or, for 137, from the kernel (memory over the limit).

**Fix:** `kubectl rollout undo`, or `kubectl replace -f` with the manifest of the repo. The user saw nothing either: the old ReplicaSet already had its pod.

## 2. Misconfigured probe

A probe asks the container a question at an interval: the **readiness** probe decides whether it gets traffic, the **liveness** probe whether the kubelet restarts it, and the **startup** probe holds the other two until the app has started. The same mistake, a wrong port, does very different things depending on the probe:

| Provoked | What Kubernetes did | What the user saw |
|---|---|---|
| Liveness of `productcatalogservice` on port 3551 instead of 3550 | Rollout "successfully rolled out" in 1 s: readiness was fine. Then it killed the healthy container every ~30 s (3 failures × 10 s), and after 4 restarts went into `CrashLoopBackOff` | 503 for 1.0 to 2.8 s at each kill, then 68.4 s in a row with the 40 s back-off, and longer each time |
| Readiness of `productcatalogservice` on port 3551 | Pod `Running` but `0/1`, no restarts, `ready=false` in its EndpointSlice, rollout stuck | Nothing: the old pod kept serving |
| Liveness of `adservice` with no delay, every 2 s and restart at the first failure | The JVM needs ~7 s to listen and was killed at 2-3 s: 3 restarts in 20 s, then `CrashLoopBackOff` of an app with nothing wrong | Nothing here (the ad is optional and the old pod kept serving); on a service of the critical path, an outage |

```sh
kubectl patch deployment productcatalogservice -n boutique \
  -p '{"spec":{"template":{"spec":{"containers":[{"name":"server","livenessProbe":{"grpc":{"port":3551}}}]}}}}'
```

**The trap:** a successful rollout only means that the new pods got ready once. A wrong liveness probe shows up afterwards, as restarts.

**Signals:**

- Events `Liveness probe failed: timeout: failed to connect service "10.244.2.33:3551" within 1s` (the wrong port is in the message) and `Container server failed liveness probe, will be restarted`.
- The restart is in place: same pod, same IP, `RESTARTS` going up. Each one cost the user 1.0 or 2.6-2.8 s: `frontend`'s gRPC client tried to reconnect while the container was restarting, was refused, and waited 1 s, then 1.6 s more, before trying again (fault 4).
- For `adservice`, `logs --previous` is empty (killed before it wrote a line) and the exit code is 143: the kubelet stopped it, the app didn't fail.

**Fix:** the right port, and for a slow start a startup probe instead of a long `initialDelaySeconds`. With `startupProbe` (gRPC every 2 s, up to 30 failures, so 60 s to start) in front of the same aggressive liveness probe, `adservice` started with no restarts. It still took 25 s to get ready, though it listened at 7 s: its readiness probe waits 20 s before the first check and then checks every 15 s. Until a pod is marked ready it gets no traffic, even if it could already serve, and that adds to the recovery time when it replaces a pod that died (fault 3; probe review, Task 9).
