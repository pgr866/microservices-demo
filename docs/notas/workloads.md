# Workloads, probes and lifecycle

The first block of Task 4: Deployments and ReplicaSets, rollouts and rollbacks, probes, and how a pod stops. Everything was done by hand with `kubectl` on the kind cluster, with one replica per service and no mesh. The faults provoked are in the [fault catalog](../fault-catalog.md) (1 to 4), with their commands and signals. This note keeps the model, the measurements side by side, and what they mean for the experiment.

## What it is

**Deployment, ReplicaSet, pod.** A Deployment holds the pod template and the number of replicas, but runs no pods itself. Each version of the template gets a ReplicaSet, named after a hash of the template (the `pod-template-hash` label of its pods), which keeps that number of pods alive: a deleted pod is replaced at once. A change to the template is a rollout: a new ReplicaSet scales up and the old one scales down to 0. Old ReplicaSets are kept (10 by default) for `kubectl rollout undo`, which turns an old template into a new revision.

```text
deployment/productcatalogservice
└── replicaset/productcatalogservice-8994577d9      one per version of the template
    └── pod/productcatalogservice-8994577d9-ldk4b
```

**Rolling update.** The default strategy allows 25 % more pods than wanted (`maxSurge`, rounded up) and 25 % fewer ready ones (`maxUnavailable`, rounded down). With one replica that is 1 and 0: the new pod starts first, and the old one stops once the new one is ready. A rollout that doesn't progress (an image that doesn't exist, a pod that never gets ready, a crash at start) leaves the old pod serving. After `progressDeadlineSeconds` (600 s) the Deployment reports `ProgressDeadlineExceeded` and `kubectl rollout status` fails, but nothing is rolled back.

**Probes.** The kubelet of each node checks its containers; by default every 10 s, with a 1 s timeout, and acting after 3 failures in a row:

| Probe | What a failure does |
|---|---|
| Startup | Until it passes, the other two don't run; if it keeps failing, the container is restarted |
| Readiness | The pod is marked not ready and leaves the EndpointSlices of its Services: no traffic, no restart |
| Liveness | The container is restarted in place: same pod, same IP |

**How a pod stops.** Deleting a pod sets its `deletionTimestamp`, and two things start at the same time: the control plane takes the pod out of the EndpointSlices, so Cilium stops sending it new connections, and the kubelet runs the `preStop` hook, sends SIGTERM, and sends SIGKILL if the container is still running when `terminationGracePeriodSeconds` runs out (30 s by default, counted from the start, so the `preStop` hook uses part of it). On SIGTERM the services stop listening, wait up to 10 s for calls in flight, and exit. A container that stops by itself (a crash, a failed liveness probe, memory over the limit) is restarted in the same pod, and if it keeps stopping, the kubelet waits longer each time, from 10 s to 5 min: `CrashLoopBackOff`.

## How it fails

What the user (the [`watch`](../../kind/README.md#watch-the-shop) pod, ten requests a second to a product page) saw in each case:

| Event | The user saw | Why |
|---|---|---|
| Rollout | 503 for 1.0 s, in 4 of 5 | The old pod stops listening before Cilium stops sending it connections |
| Rollout, with a 5 s `preStop` pause | Nothing (2 of 2) | The old pod serves while it is taken out |
| Rollout of a version that crashes, has no image or never gets ready | Nothing | The old pod stays |
| `kubectl delete pod` | 503 for 2.4-2.6 s | New pod ready in ~1 s, then the reconnect backoff |
| `kubectl delete pod`, with the pause | Nothing (2 of 2) | |
| `--grace-period=0 --force` | 503 for 1.0-2.7 s | The app still gets SIGTERM: same as a normal delete |
| `--grace-period=0 --force`, with the pause | One request failed | The pause is cut after ~1 s, SIGKILL 2 s after SIGTERM |
| `kill -9` of the process | 503 for 1.0 s | The container restarts in the same pod in 0.6 s |
| `kubectl delete pod` of `frontend` | No answer for 11.3-12.0 s | Its readiness probe waits 10 s before the first check |
| Liveness probe on a wrong port | 503 for 1-3 s every ~30 s, then 68 s in a row | Healthy container killed until `CrashLoopBackOff` |
| `cartservice` at 0 replicas for 26 s / 210 s | 503 until 19 s / 40 s after the new pod was ready | gRPC reconnect backoff, up to 120 s between attempts |

Three lessons come out of it:

1. **"Successfully rolled out" only means the new pods got ready once.** A wrong liveness probe passes the rollout and restarts the pod afterwards. A wrong readiness probe, or a crash at start, stops the rollout before it does harm.
2. **Kubernetes recovers before the user does.** The readiness delays (`initialDelaySeconds`: 10 s in `frontend`, 15 s in `cartservice`, 20 s in `adservice`) and the reconnect backoff of the gRPC clients come on top of the time a pod takes to start. Every 1.0 s and 2.6 s in the table is one or two refused reconnect attempts (1 s, then 1.6 s more).
3. **Changes made with `kubectl set`, `kubectl patch` or `rollout undo` drift from git, and part of the drift leaves no trace.** `kubectl apply` merges three versions (the file, the last one applied and the live object) and only removes what an earlier apply put there: a field the file writes is put back (a changed port, a removed variable), and `kubectl diff` shows it; a field the file doesn't write (an added variable, a probe interval left at its default) stays, and `kubectl diff` shows nothing. `kubectl replace -f` writes the whole manifest and brings it back. Argo CD diffs and syncs with the same three-way merge by default, so whether it sees the second kind is to be checked in Task 13.

## How it is measured

- **What the user sees:** the `watch` pod ([`kind/watch/`](../../kind/watch/watch.js)), which prints the time of each change of answer and how long the previous one lasted.
- **What Kubernetes says:** `kubectl rollout status` and `history`; `kubectl describe pod` (`Last State`, `Exit Code`, events); `kubectl logs --previous`, the logs of the container that stopped; the conditions of the pod (`Ready` and when it changed) and its EndpointSlices.
- **The exit code** says who stopped a container: 1 is the app, 143 is SIGTERM and 137 SIGKILL (the kubelet, or the kernel for memory). The JVM of `adservice` returns 143 on every normal stop.
- **The kubelet's metrics:** `/api/v1/nodes/<node>/proxy/metrics/probes` has the duration of each probe (`prober_probe_duration_seconds`) and the count by result; `.../metrics/cadvisor` has the CPU periods throttled by the limit.
- **On the node,** for a container whose pod is already gone from the API: `docker exec <node> crictl inspect <id>` gives its exit code, finish time and PID (to SIGKILL it).

## What it means for the experiment

- **E2's kill is still to be decided.** Chaos Mesh's `pod-kill` with no grace period is not a SIGKILL: the apps get SIGTERM and stop in order, as with a normal delete, and in C and D it cuts the `preStop` pause. A real crash is `container-kill`, which restarts the container in the same pod: a different recovery (AGENTS.md, Task 4).
- **Both recovery times are needed.** Measured only until the pod is ready, every outage in the table would look shorter than the user saw it; the gap is the gRPC backoff, from 1 s to 2 min. With two replicas (C and D) and with the mesh, it may disappear: to check in those blocks.
- **The `preStop` pause of the resilience factor works even with one replica:** no errors in rollouts or deletes. A forced kill skips it.
- **`terminationGracePeriodSeconds` is the 30 s default in the ten services** (seven had 5 s, which left no room for the drain of the code or for the pause). It is a limit, not a wait.
- **Probe review (Task 9):** the readiness delays add straight to the outage with one replica, and the first liveness check of `cartservice` fails at every start with its 300m CPU limit, which only B and D set. Where its fix goes is still to be decided.
