# Fault catalog

Each fault is provoked on purpose in the kind cluster, diagnosed and written down: how it was caused, what the user saw, the signals that give it away, and how it was fixed. The catalog grows with each block of Task 4. Only two or three of these faults go into the measurement campaign; the rest are qualitative analysis for the thesis.

"The user" is the [`watch`](../kind/README.md#watch-the-shop) pod, which loads a product page ten times a second. Times are from the cluster of 2026-11-27: one replica per service, the manifests of [`kubernetes-manifests/`](../kubernetes-manifests/README.md).

| # | Fault | Block | Status |
|---|---|---|---|
| 1 | [CrashLoopBackOff](#1-crashloopbackoff) | Workloads | Done |
| 2 | [Misconfigured probe](#2-misconfigured-probe) | Workloads | Done |
| 3 | [Forced pod kill](#3-forced-pod-kill) | Workloads | Done |
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

## 3. Forced pod kill

E2 will kill a pod the way Chaos Mesh does, deleting it with no grace period. Here it is next to a normal delete and a crash, on `productcatalogservice` (Go, one replica), and on `frontend`:

| How | What the app got | Recovery | The user saw |
|---|---|---|---|
| `kubectl delete pod` | SIGTERM: it drained and exited with 0 in under 0.5 s | New pod, ready in ~1 s | 503 for 2.6 s, 2.4 s |
| `kubectl delete pod --grace-period=0 --force` | SIGTERM as well, exit code 0 | New pod, ready in ~1 s | 503 for 2.7 s, 1.0 s |
| `kill -9` of the process, from the node | Nothing: exit code 137 | The same pod restarted its container in 0.6 s, same IP | 503 for 1.0 s |
| `kubectl delete pod` of `frontend` | SIGTERM | New pod, ready in 12 s (`initialDelaySeconds: 10` of its readiness probe) | No answer for 12.0 s, 11.3 s |

```sh
kubectl delete pod <pod> -n boutique --grace-period=0 --force
docker exec <node> crictl inspect -o go-template --template '{{.info.pid}}' <container-id>   # then: docker exec <node> kill -9 <pid>
```

**`--grace-period=0` is not a SIGKILL.** It removes the pod from the API at once, without waiting for the kubelet, so the ReplicaSet creates the new pod at once too. On the node, though, the kubelet still sends SIGTERM and gives the process a short grace period (2 s here) before SIGKILL. Our services drain in milliseconds, so for them it is the same as a normal delete. A crash the app doesn't see coming is a SIGKILL of the process (Chaos Mesh's `container-kill`), and Kubernetes recovers from it in another way: the same pod restarts its container, with no new pod and no new IP.

**The user waits longer than Kubernetes.** The new pod was ready in ~1 s, but the user got errors for 1.0 or 2.6 s: the steps of the reconnect backoff of `frontend`'s gRPC client (1 s, then 1.6 s more; fault 4). The two recovery times of the experiment, until the pod is ready and until the user is served, already differ here.

**The old pod closes too early.** On SIGTERM the app stops listening at once, while for a few milliseconds Cilium still sends it new connections, which are refused. That is why 4 of 5 rollouts cost the user 1.0 s of 503, though the new pod was already ready. A `preStop` pause keeps the old pod serving while it is taken out of the Service (`lifecycle.preStop.sleep.seconds: 5`, with 30 s of grace):

| With the pause | The user saw |
|---|---|
| Rollout | Nothing (2 of 2) |
| `kubectl delete pod`, with one replica | Nothing (2 of 2): the old pod served during the pause while the new one got ready |
| `--grace-period=0 --force` | One request failed after 1.2-1.7 s (2 of 2). The kubelet cut the pause after ~1 s, sent SIGTERM, and SIGKILL 2 s later (exit code 137): the app didn't finish draining in time |

The pause belongs to the resilience factor (C and D); a forced kill skips it, which is what E2 wants.

**`terminationGracePeriodSeconds`:** seven services had 5 s. The apps stop in milliseconds, so 5 s cut nothing here, but it left no room: the code waits up to 10 s for calls in flight, sized for the 30 s default, and a `preStop` pause counts within the grace period. The manifests now leave the 30 s default in the ten services. It is a limit, not a wait, so it delays nothing.

**Java exits with 143.** `adservice` ends with exit code 143 on every stop, also when its shutdown finishes cleanly: it is what the JVM returns after SIGTERM. While the old pod is removed, `kubectl get pods` shows it as `Error`, which is not a failure.
