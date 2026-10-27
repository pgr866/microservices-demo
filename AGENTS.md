# TFM — Memoria de contexto y hoja de ruta

> Documento único de contexto para retomar el trabajo. Recoge las decisiones vigentes y su motivo, para no volver a discutirlas sin un motivo nuevo. El detalle histórico está en git.

## 0. Resumen del proyecto

**Título:** Evaluación de medidas de seguridad y resiliencia en la orquestación de microservicios con Kubernetes: un análisis experimental de rendimiento, aislamiento y recuperación ante fallos.

**Objetivo personal: aprender Kubernetes a fondo.** El objetivo académico es el del título, y el experimento es el vehículo del aprendizaje: cada concepto se estudia, se rompe a propósito y se mide.

**Sistema bajo prueba:** fork de [Online Boutique de Google](https://github.com/GoogleCloudPlatform/microservices-demo): 10 microservicios en 5 lenguajes (el `loadgenerator` original se sustituye por k6).

**Stack:** Azure (AKS), Terraform, GitHub Actions (OIDC), Argo CD, Kubernetes + Cilium, Istio, `kind`, Kustomize, Kyverno, Chaos Mesh, Prometheus, Grafana, k6, Trivy, cosign.

**Restricciones:** 3 meses a dedicación casi completa y **100 € de Azure**. El 90 % del trabajo va en `kind` (gratis); Azure solo para validar la infraestructura y para la campaña de medida.

### Principios de trabajo

1. **Primero a mano, luego con herramientas:** cada concepto de Kubernetes se hace una vez con `kubectl` y YAML crudo antes de usar Kustomize, Kyverno o Argo CD.
2. **Aprender rompiendo:** cada tema acaba en un fallo provocado y diagnosticado (catálogo de fallos, Tarea 12).
3. **Cada tema se escribe:** una nota corta en `docs/notas/` (qué es, cómo falla, cómo se mide). Es la base de la memoria y material de portfolio.
4. **CI, CD y Terraform no se recortan.** Crecen en paralelo con cada fase de Kubernetes, no en un bloque aparte.
5. **Decisiones en ADRs** (`docs/adr/`), con el motivo.

---

## 1. Diseño experimental

### Factores (diseño 2×2)

| | Resiliencia OFF | Resiliencia ON |
|---|---|---|
| **Seguridad OFF** | **A** base | **C** resiliencia |
| **Seguridad ON** | **B** seguridad | **D** ambas |

Cada factor es un *component* de Kustomize; cada escenario, un overlay con su namespace (`boutique-a`, `-b`, `-c`, `-d`).

- **Service mesh (Istio) en los 4 escenarios** (petición del tutor): los 4 pagan el mismo coste de proxies y lo que cambia es su configuración según el factor. Si estuviera solo en C y D, su latencia se confundiría con el efecto de la resiliencia; como tercer factor (2×2×2) duplicaría la campaña. Configuración común: **reintentos desactivados explícitamente** (Istio reintenta 2 veces por defecto y ocultaría los fallos inyectados) y reparto por petición (con gRPC, sin mesh, `frontend` mantiene una sola conexión HTTP/2 por servicio y puede mandarlo todo a una réplica).
- **Seguridad:** Pod Security Admission `restricted`; NetworkPolicy con deny-all más reglas permitidas; requests y limits por servicio, `ResourceQuota` y `LimitRange`; `seccompProfile: RuntimeDefault`; `automountServiceAccountToken: false`; Kyverno (pocas políticas, incluida `verifyImages`) limitado a los namespaces con seguridad ON; mTLS `STRICT` y `AuthorizationPolicy` por identidad del mesh (la NetworkPolicy aísla por IP y puerto; esta, por identidad del servicio). El cifrado no es lo que distingue los escenarios: Istio cifra por defecto el tráfico entre pods del mesh en los 4 (modo `PERMISSIVE`); `STRICT` añade rechazar el texto plano de quien no está en el mesh.
- **Resiliencia:** 2 réplicas por servicio sin estado (`redis-cart` se queda en 1: con 2 se partirían los carritos); PDB; `topologySpreadConstraints`; probes revisadas; `preStop` + `terminationGracePeriodSeconds`; **circuit breaker** del mesh (`DestinationRule` con `outlierDetection` y `connectionPool`). Kubernetes solo saca un pod del tráfico cuando falla su readiness probe; el circuit breaker expulsa la réplica que da errores en peticiones reales aunque siga `Ready` (*fallo gris*).
- **En A y C la app va sin requests ni limits.** El pod ya no es BestEffort, porque el sidecar lleva sus propios recursos: se fijan iguales en los 4 escenarios, así que no distinguen un factor de otro; lo que importa para H3 es que el contenedor de la app no tenga límite. `kustomize/base` los trae en todos los Deployments (es el original, aún sin tocar), así que pasan de la base al componente de seguridad con sus valores por servicio; los valores por defecto del `LimitRange` no bastan (p. ej. para la JVM de `adservice`). Sin eso, el experimento de contención de recursos no tiene sentido.
- **Mismo SHA de imagen en los 4 escenarios** durante todo el experimento.
- **El `ResourceQuota` es idéntico en B y D**, dimensionado para 2 réplicas y con los proxies del mesh incluidos (cuentan dentro de la quota).

### Hipótesis

- **H1:** la seguridad (red, mTLS `STRICT`, limits y quotas, admisión) no degrada de forma relevante latencia (p95) ni consumo. Se contrasta con un test de equivalencia (TOST, con el margen fijado antes de medir, p. ej. ±10 % del p95) o tamaño del efecto con IC; un resultado nulo es válido. Los limits de CPU pueden provocar *throttling*: si H1 se rechaza, también es un resultado.
- **H2:** la seguridad bloquea el 100 % del movimiento lateral (un pod no autorizado llega a `redis-cart` en A y C, no en B y D; lo frenan la NetworkPolicy y la `AuthorizationPolicy`).
- **H3:** limits y quotas mitigan la interferencia por contención de recursos (*noisy neighbor*): la degradación del p95 del usuario testigo es mucho menor en B y D que en A y C.
- **H4:** (a) la resiliencia reduce el MTTR y los errores durante el fallo; (b) el efecto de la seguridad sobre el MTTR **se mide, no se da por nulo** (los webhooks de admisión y `verifyImages` añaden latencia al recrear pods); (c) el circuit breaker reduce los errores durante un fallo gris (E2b, C frente a C′).

### Experimentos

- **E1, rendimiento:** carga escalonada sin fallos. **Variante A0** (A sin mesh, solo en E1): como el mesh está en los 4, sin ella no se sabe cuánto cuesta el propio mesh, y es lo primero que preguntará el tribunal.
- **E2, fallo:** `PodChaos` con kill inmediato (`grace-period=0`) sobre un servicio **sin estado y en el camino caliente** (`currencyservice` o `productcatalogservice`; fijarlo antes de medir) mientras corre el usuario testigo. `redis-cart` no sirve de objetivo (SPOF en todos los escenarios): queda como fallo cualitativo aparte.
- **E2b, fallo gris:** se degrada **una sola** réplica del mismo servicio objetivo sin que deje de estar `Ready` (`NetworkChaos` de Chaos Mesh sobre ese pod, **limitado al tráfico de sus llamantes**: si también afecta a las probes del kubelet, el pod deja de estar `Ready` y ya no es un fallo gris). **El fallo tiene que ser uno que Envoy cuente como error** (conexión rechazada o cortada, p. ej. partición o pérdida alta), no solo un retraso: si la petición la corta el plazo de `frontend`, Envoy ve una cancelación del cliente y puede no contarla para expulsar la réplica. Se verifica en el piloto que la expulsión ocurre. Sin circuit breaker, parte de las peticiones siguen yendo a la réplica mala; con él, se expulsa. E2 mide la recuperación de Kubernetes; E2b, la del mesh. **Variante obligatoria C′** (C sin circuit breaker, solo en E2b): sin ella, H4c no se puede contrastar, porque A y B frente a C y D también difieren en el número de réplicas. H4c se contrasta con C frente a C′; A, B y D se miden igual para el análisis del factorial.
- **E3, aislamiento:** (i) pod no autorizado que intenta llegar a `redis-cart`; (ii) pod `stress-ng` colocado en el mismo nodo que la víctima. El manifiesto es idéntico en los 4 escenarios; los límites los pone el `LimitRange` donde exista. Sus imágenes salen de ACR firmadas y el manifiesto cumple PSA `restricted`, para que en B y D no los frene la admisión y se mida solo la red o los límites.

### Metodología

- **Carga:** k6 con `constant-arrival-rate` (no VUs fijos: con VUs fijos, cuando el sistema cae la latencia se subestima, *coordinated omission*), calibrada al ~60 % de la saturación del escenario A y con la misma tasa en los 4. La saturación se calibra en AKS, no en `kind`.
- **Repeticiones:** N = 5 por escenario y experimento (3 + 2 entre las dos sesiones), con calentamiento y duración fija; orden alterno; se redespliega el namespace entre ejecuciones (resetea Redis).
- **Análisis:** efectos principales de cada factor y su interacción con un ANOVA de rangos alineados (ART, no paramétrico), con la sesión como bloque; comparaciones por pares con Mann-Whitney, con corrección de Holm por comparaciones múltiples; medianas e IC. Con N = 5 la potencia es baja: se informa siempre del tamaño del efecto con su IC, y un TOST no concluyente no se presenta como equivalencia. En un notebook de Python.
- **Bloqueo por sesión:** las dos sesiones de campaña repiten **los 4 escenarios** cada una (las VMs cambian de un día a otro; si no, sesión y escenario se confunden).
- **MTTR doble:** hasta pod `Ready` (kube-state-metrics) y hasta que el éxito del usuario testigo vuelve a la línea base (k6).
- **Autoescalado fuera del factorial:** nodos fijos y Cluster Autoscaler apagado durante la campaña; el HPA es una demo aparte.
- **Sin interferencia de medida:** node pools separados (`sistema`: Argo CD, Prometheus, Grafana, Chaos Mesh; `carga`: k6; `app`: el sistema bajo prueba, con taints) y los otros 3 namespaces escalados a 0 mientras se mide uno.
- **Fijado antes de medir e igual en los 4:** `REQUEST_TIMEOUT`; `MAX_WORKERS` de los servicios Python; llamadas en paralelo por producto (máx. 10, `errgroup`); sin reintentos automáticos, tampoco en el mesh (ocultarían fallos inyectados); parámetros del circuit breaker; servicio objetivo; tipo de fallo (kill inmediato, ver E2; un `delete` normal sería otro experimento, porque con el apagado ordenado el pod termina antes sus peticiones en curso).
- **k6 corre dentro del clúster**, en su propio namespace y en el pool `carga`, con recursos fijos e iguales para los 4 escenarios (dentro de B o D le aplicarían el `LimitRange` y la quota). Ataca al `ClusterIP` de `frontend` (`frontend.<namespace>.svc.cluster.local`), nunca por la entrada pública (añadiría latencia del Gateway, DNS y TLS); las NetworkPolicy de B y D admiten su tráfico. **El namespace de k6 entra en el mesh** (con sidecar, igual en los 4): si no, con mTLS `STRICT` en B y D sus peticiones en texto plano se rechazarían. Lo mismo aplica al Job de `e2e/` y al Gateway, que llegan a `frontend` desde fuera de su namespace.
- **Imágenes ya descargadas** en los nodos de `app` antes de medir, incluidas las del mesh (p. ej. con un DaemonSet de *pre-pull*), para que el MTTR no incluya la descarga.

---

## 2. Estado actual y calendario

- ✅ **Tareas 1-3:** fork limpio y los 10 microservicios validados en local (tests, linter, formato, Dockerfiles endurecidos, entorno de desarrollo). Código de servicios **congelado**: no se añade nada nuevo.
- ✅ **Tests de punta a punta en `e2e/`** (falta engancharlos al CI).
- ⏳ **Tareas 4-14:** diseñadas, sin implementar.
- **Rama de trabajo:** `refactor/local-microservice-validation`; `main` no se toca hasta fusionar por PR.
- **Orden:** todo lo que no cuesta dinero primero (`kind`, CI, `terraform plan`). El primer `terraform apply` va en la semana 7; antes solo se crean, con `az`, el almacenamiento del estado y la identidad OIDC (Tarea 5).

| Sem. | Kubernetes y experimento | CI / CD / Terraform en paralelo |
|---|---|---|
| 1 | Hipótesis y ADRs, repaso del modelo, `kind` multinodo con Cilium. Visto bueno del tutor | Auditar el CI heredado y diseñar el nuevo. Terraform: estructura de módulos y bootstrap (estado e identidad OIDC) |
| 2 | Workloads, probes y ciclo de vida a mano. Overlay A | CI track A (tests, cobertura, linters) en matriz |
| 3 | Recursos, QoS, throttling, OOM. Overlay B | CI track A completo (caché de BuildKit, `go test -race`) |
| 4 | Scheduling, PDB, `drain`. Factor resiliencia (C y D). Reproducir sin mesh la réplica desequilibrada de gRPC y el fallo gris | CI track B (Trivy, `govulncheck`, e2e, `kustomize` + `kube-linter`, `actionlint`) |
| 5 | Red: CoreDNS, NetworkPolicy, Cilium. Istio (sidecar) en `kind` con Cilium configurado como AKS, circuit breaker | Terraform: VNet, AKS con 3 pools, ACR, OIDC. `fmt`, `validate`, `plan`, tflint, checkov, `trivy config` |
| 6 | Seguridad: RBAC, PSA, Kyverno, mTLS `STRICT` y `AuthorizationPolicy`. Prometheus y Grafana en `kind` | Pipeline de Infra: levantar y destruir el entorno bajo demanda |
| 7 | **Sesión de Azure 1:** apply, comprobar métricas de Cilium, el mesh sobre el Cilium de AKS y la observabilidad, smoke, destroy | Pipeline de CD: ACR, tag por SHA, Trivy, `cosign` y SBOM, gate manual |
| 8 | Argo CD con `ApplicationSet` de 4 apps; Chaos Mesh, scripts de k6, piloto en `kind` (E2b y C′ incluidos); métricas personalizadas cerradas; `verifyImages` en `kind` (registro local) | Tramo GitOps del CD en `kind` (Argo CD tira de git; los runners de GitHub no llegan a un `kind` local) |
| 9 | **Sesión de Azure 2:** CD de extremo a extremo, Gateway API + cert-manager + TLS, calibración de carga en AKS | Pipeline de Experimento (`workflow_dispatch`) |
| 10 | **Campaña en AKS** (sesión 1 de 2: los 4 escenarios) | El pipeline de Experimento la ejecuta |
| 11 | **Campaña** (sesión 2 de 2), estadística y gráficas | Documentación de pipelines |
| 12 | Memoria final, README, ADRs, portfolio | Buffer |

El catálogo de fallos (Tarea 12) se construye de forma incremental en las semanas 2-6. La memoria lleva un borrador casi completo antes de la semana 11.

**Si el tiempo aprieta, cede en este orden** (nunca CI, CD ni Terraform, ni el circuit breaker, que pidió el tutor): N de 5 a 3 → catálogo de fallos solo cualitativo → `AuthorizationPolicy` (la seguridad de red queda en NetworkPolicy + mTLS `STRICT`) → factorial 2×2 reducido a A vs D (con D′, D sin circuit breaker, en lugar de C′ en E2b) → variante A0 → mini-operador y certificaciones (opcionales).

**Semanas 4-6 son las más cargadas** (resiliencia, red, seguridad, mesh y Terraform a la vez). Si al cerrar la semana 6 Istio no funciona en `kind`, se aplica ya el orden de recorte de arriba y no se retrasa la sesión de Azure 1.

**Plan B de Azure:** si se acaba el presupuesto, la campaña se repite en `kind` y se documenta como limitación.

### Presupuesto (≈ 4 VMs `D2s_v5`, ~0,4 $/h; confirmar con la calculadora de Azure)

Los proxies del mesh pueden exigir un nodo más en `app` (~5-8 € más en la campaña, a cargo del margen); se confirma en el piloto.

| Concepto | Estimación |
|---|---|
| Sesiones de desarrollo en AKS (~50 h) | ~20 € |
| Campaña (~40 h) | ~15 € |
| ACR Basic, Blob de estado, IP, discos | ~20 € |
| Margen | ~45 € |

Un clúster 24/7 serían ~280 $/mes: **se destruye entre sesiones** (`terraform destroy`; un clúster parado sigue cobrando discos e IP). Se conservan solo el estado de Terraform y ACR. Alertas de presupuesto al 50 % y 80 % definidas en Terraform. Máximo una sesión de Azure por semana.

---

## 3. Sistema bajo prueba (Tareas 1-3, hecho)

Se modifica el fork en vez de copiar `src/` a un repo nuevo: los manifiestos y `kustomize/` ya existen y el historial de git es evidencia citable del proceso. Todo lo exclusivo de Google Cloud se eliminó. **`helm-chart/`, `terraform/` (GCP) y `src/loadgenerator/` (Locust) siguen solo como referencia y se borran antes de publicar el repo**, con lo que dependa de ellos: Locust cuando exista el script de k6 (sus manifiestos salen antes, en la Tarea 8); el Terraform de GCP se sustituye en `terraform/` por el de Azure; `helm-chart/` junto con `helm-chart-ci.yaml` y sus referencias en los otros workflows.

### Reglas para cualquier cambio en un servicio

1. Compila, arranca y pasa sus tests.
2. Sin código específico de GCP ni OpenTelemetry (no hay Collector; la observabilidad es la de la Tarea 14).
3. Sin código muerto: funciones sin uso, ramas inalcanzables, variables de entorno que nadie lee.
4. **Tests solo de lógica real y alcanzable sin red**, con fakes en memoria. Lo que exige abrir un puerto queda sin cubrir a propósito. Las pruebas con los servicios reales van en `e2e/`.
5. Código generado (`genproto/`, `demo_pb2*.py`, `obj/`, `build/generated`) fuera del linter y de la cobertura. Nunca se tocan las cabeceras de licencia.
6. **Dockerfile de producción:** base fijada por digest; imagen final mínima; usuario numérico no-root (`65532` en distroless, `1000` en Alpine); `HEALTHCHECK NONE` (la salud la comprueban las probes); ningún `COPY` con `--chown` en el stage final.
7. **`Dockerfile.dev` multietapa**, con las mismas etapas en los 10 servicios: `test`, `coverage`, `lint`, `format`, `format-check`, `dependencies`, `codegen` y, en Go, `vulncheck` (`cartservice` no tiene `format-check` ni `codegen`). Se ejecutan con `docker build -f Dockerfile.dev --target <etapa> .` y lo generado sale con `--output type=local,dest=.`. La última etapa, `dev`, es el hot-reload de Compose. **El CI usa estas mismas etapas**; nada de `setup-<lenguaje>`.

### Herramientas y umbrales de cobertura (los usa el CI)

| Servicio | Lenguaje | Tests | Linter | Umbral |
|---|---|---|---|---|
| `productcatalogservice` | Go | `go test` | golangci-lint | 45 % |
| `shippingservice` | Go | `go test` | golangci-lint | 55 % |
| `checkoutservice` | Go | `go test` | golangci-lint | 70 % |
| `frontend` | Go | `go test` | golangci-lint | 70 % |
| `paymentservice` | Node | Jest | ESLint + Prettier | 70 % |
| `currencyservice` | Node | Jest | ESLint + Prettier | 70 % |
| `emailservice` | Python | pytest | ruff | 70 % |
| `recommendationservice` | Python | pytest | ruff | 70 % |
| `adservice` | Java | JUnit + JaCoCo | Checkstyle + google-java-format | 50 % |
| `cartservice` | C# | xunit v3 + Coverlet | `dotnet format` | 70 % |

Los umbrales por debajo del 70 % son de servicios donde casi todo lo no cubierto es arranque del servidor; se prefiere bajar el umbral a excluir código propio del cómputo.

**Hallazgo intencionado del linter:** cada servicio deja a propósito **un único** hallazgo (una expresión booleana redundante), para demostrar que el linter funciona en CI. No corregirlo; documentarlo en el README para que no se lea como descuido.

| Stack | Regla | Dónde |
|---|---|---|
| Go | `S1002` | lectura de `PORT` en `main` |
| Java | `SimplifyBooleanExpression` | `blockUntilShutdown()` |
| Node | `no-extra-boolean-cast` | `charge.js` (payment), guard `require.main` (currency) |
| Python | `SIM210` | `logger.py` (email), `start()` (recommendation) |
| C# | `WHITESPACE` | `RequireUserId(...)` de `EmptyCart` |

### Decisiones de comportamiento que afectan al experimento

- **Puertos** (iguales en desarrollo y producción): `frontend` 8082, `paymentservice` 50052, `recommendationservice` 8081, `shippingservice` 50051, `emailservice` 8080 (su `Service` publica 5000: igualar a 8080, Tarea 8).
- **Errores gRPC**, iguales en todos los servicios: petición inválida → `InvalidArgument`; estado del sistema (carrito vacío) → `FailedPrecondition`; dependencia caída o saturada → `Unavailable`; otro fallo → `Internal`. Excepciones: `NotFound` en `productcatalogservice` y `DeadlineExceeded`/`Canceled` en `checkoutservice` solo cuando vence o se cancela el contexto del llamante. El código de una dependencia nunca se reenvía y nunca viaja una traza al cliente. `frontend` traduce a HTTP con la tabla de `google/rpc/code.proto`; en un 5xx muestra un mensaje genérico y el *request ID*.
- **Plazos:** `frontend` corta cada petición a `REQUEST_TIMEOUT` (por defecto `5s`) y el plazo se propaga por toda la cadena. Sin reintentos automáticos en ningún cliente ni en el mesh.
- **Apagado ordenado con SIGTERM** en los 10 servicios: dejan de aceptar peticiones, el health check pasa a `NOT_SERVING`, terminan las peticiones en curso en 10 s (por debajo de los 30 s de Kubernetes) y cortan el resto.
- **Logs:** JSON con `timestamp`, `severity` y `message`; `LOG_LEVEL` = `debug`, `info`, `warn` o `error`. **Nunca** en los logs: número de tarjeta, CVV, email ni ID de sesión (hay tests de regresión).
- **`cartservice` / Redis:** cada carrito es un hash `cart:<usuario>`; `HINCRBY` + `EXPIRE` de 48 h en una transacción (atómico con varias réplicas). Con Redis caído falla al instante (`FailFast`). Su health check no depende de Redis, para que un Redis caído no reinicie todos los pods. Sin `REDIS_ADDR` usa un almacén en memoria. Redis sin contraseña a propósito: limitar quién llega a él es el eje de la NetworkPolicy.
- **`frontend` (expuesto a internet):** CSP estricta, cabeceras de seguridad, CSRF con `http.NewCrossOriginProtection()` (k6 y curl pasan), cookies `HttpOnly`/`SameSite=Lax`/`Path=/` (`Secure` y HSTS cuando haya TLS, Tarea 10), timeouts del servidor y límite de 64 KiB al cuerpo.

### Escaneo de vulnerabilidades

`trivy image --scanners vuln,secret --image-config-scanners misconfig` sobre las 10 imágenes (hoy: 0 vulnerabilidades, 0 secretos, 0 *misconfigurations*), más `govulncheck` en los 4 servicios Go. Versiones fijadas: Trivy `0.76.0`, k6 `2.3.0`, `redis:8.10.2-alpine` (digest en `compose.yaml`); el resto, en los Dockerfile.

### Entorno de desarrollo

`compose.yaml` levanta los 10 servicios más `redis-cart` con hot-reload (etapa `dev`); devcontainers por servicio en `.devcontainer/<servicio>/`. Gotchas:
- Construir un devcontainer exige `BUILDX_BAKE_ENTITLEMENTS_FS=0` en el host.
- Tras cambiar `package.json`: `docker compose down -v <servicio>` y luego `up --build`.
- `adservice`: parar su contenedor de desarrollo antes de lanzar las etapas de Gradle (comparten el bloqueo de `.gradle/`).
- En Windows, el repo se clona dentro de WSL2.

---

## Tarea 4 — Kubernetes en `kind` ⏳

Aquí se hace el grueso del aprendizaje de Kubernetes (semanas 1-6), con YAML a mano antes de Kustomize.

- Clúster `kind` **multinodo** (1 control plane + 3 workers) con **Cilium** como CNI, el mismo dataplane que AKS (Tarea 7), para que las NetworkPolicies y sus métricas se comporten igual en local y en la nube. Comprobar que la máquina aguanta 4 nodos más Prometheus.
- Las imágenes de `kustomize/base` siguen siendo las de Google: construir las propias y cargarlas con `kind load docker-image`.
- Cada bloque de temas termina con su nota en `docs/notas/` y su fallo provocado: workloads y ciclo de vida; recursos y QoS (throttling, OOM); scheduling (taints, affinity, PDB, `drain`); red (Services, CoreDNS, NetworkPolicy, Cilium); seguridad (RBAC, PSA, Kyverno).
- Validar los 4 overlays: los 10 servicios y `redis-cart` sanos; las NetworkPolicies bloquean lo no autorizado (acceso a Redis desde un pod sin permiso) y dejan pasar lo legítimo; un pod que excede `LimitRange`/`ResourceQuota` es rechazado.
- Ejecutar `e2e/smoke.js` y `e2e/cart-concurrent.js` en cada namespace con el Job preparado (`kubectl apply -k e2e -n <namespace>`, ver `e2e/README.md`) y una prueba de humo de la inyección de fallos y de la carga.
- **Istio** instalado por Helm, igual que irá en AKS (no el add-on de AKS, para que los dos entornos sean idénticos). Primero ver el problema sin mesh (semana 4: réplica desequilibrada con gRPC y fallo gris, ambos al catálogo de fallos) y luego añadirlo. **Modo sidecar** (decidido; ambient descartado, ver «Descartados»). Puntos a resolver:
  - **`istio-cni` obligatorio:** el *init container* por defecto pide `NET_ADMIN` y PSA `restricted` lo rechaza.
  - **Convivencia con Cilium:** si Cilium resuelve el `Service` en el *socket* del pod, el sidecar solo ve la IP del pod destino y no aplica reparto ni circuit breaker (sin dar error). En `kind`, Cilium con los ajustes que pide Istio (`socketLB.hostNamespaceOnly=true`, `cni.exclusive=false`), y comprobar que el Cilium gestionado de AKS se comporta igual. **Si no, en AKS se usa el add-on de Istio de AKS**, que Microsoft soporta sobre su dataplane (también es sidecar; las mismas CRDs).
  - **Arranque y parada:** sidecars nativos (*init containers* con `restartPolicy: Always`) para que el proxy arranque antes y pare después que la app, sin errores en el arranque ni en el apagado ordenado.
  - **Prueba de que el mesh actúa de verdad:** en el fallo gris, el circuit breaker expulsa la réplica (telemetría de Istio), no solo que los pods tengan el sidecar.
- Comprobar que las NetworkPolicy dejan pasar el tráfico del mesh (puertos del plano de control `istiod`, p. ej. 15012, y DNS) y el *scraping* de Prometheus a los sidecars (15090), y que los reintentos de Istio están a 0.
- Con mTLS `STRICT` (B y D), comprobar que siguen llegando a `frontend` k6, el Job de `e2e/` (con sidecar nativo, para que el Job termine: con un sidecar clásico no acabaría nunca) y el Gateway.
- Medir con carga, antes de decidir, los pendientes de la Tarea 8: memoria de la JVM de `adservice` y límite de streams gRPC.
- Argo CD en el mismo `kind` para validar el `ApplicationSet` (semana 8).

## Tarea 5 — Validación local de Terraform ⏳

`terraform fmt -check`, `validate` y `plan`. **Bootstrap:** un script con `az` crea la cuenta de almacenamiento del estado (con bloqueo) y la identidad OIDC de GitHub Actions: sin lo primero `plan` no funciona con backend remoto, y sin lo segundo el CI no puede ejecutarlo. Es lo único que se crea en Azure antes del primer `apply` (céntimos al mes). Solo con las Tareas 3, 4 y 5 en verde se ejecuta el primer `apply`.

---

## Tarea 6 — Pipeline de CI ⏳

Se dispara en cada PR, con dos tracks en paralelo. **Todo se ejecuta en Docker, con las etapas de `Dockerfile.dev`**: cada etapa genera el formato que espera la action de informe, y esa action anota la PR. Las actions se fijan por SHA (Renovate las actualiza). Va creciendo de alcance con cada fase.

### A) Matriz por microservicio

1. **Tests + cobertura** en una sola ejecución. Informe con `dorny/test-reporter`: `golang-json`, `java-junit` (sirve también para pytest y `jest-junit`) y `dotnet-trx`.
2. **Gate de cobertura, bloqueante**, con los umbrales de la tabla: Go → `gwatts/go-coverage-action`; Java → `madrapps/jacoco-report`; Node → `ArtiomTr/jest-coverage-report-action`; Python y .NET → Cobertura XML con `irongut/CodeCoverageSummary`. Gradle + JaCoCo es lo más frágil: si falla, se documenta como limitación.
3. **Linter, informativo**, con anotaciones: `golangci-lint-action`, `reviewdog/action-eslint`, `astral-sh/ruff-action`; reviewdog con `-f=dotnet` y `-f=checkstyle`, quitando antes el prefijo `/app/` de las rutas (`sed 's|/app/||g'`).
4. **Formato, informativo — decisión del usuario, no reconsiderar.** Usa `format-check`; el CI nunca formatea.
5. **`go test -race`** en los 4 Go, en un paso propio (necesita CGO y `gcc`): la carrera de datos corregida en `productcatalogservice` solo se detecta así.
6. **Caché de BuildKit** persistida entre ejecuciones (p. ej. `buildkit-cache-dance` con `actions/cache`); GitHub Actions no la conserva sola.

### B) Infraestructura, imágenes y manifiestos

- `docker build` de las 10 imágenes de producción + Trivy (informativo; el bloqueo está en el CD) + `govulncheck`.
- **Tests de punta a punta (`e2e/run.sh`), job bloqueante.**
- `terraform fmt`/`validate`/`plan` comentado en la PR + tflint, checkov y `trivy config` (tfsec está archivado y fusionado en Trivy).
- `kustomize build` de los 4 overlays + `kube-linter`, que además da un dato comparativo entre escenarios. `kustomize-build-ci` hoy solo compila `kustomize/` y `kustomize/tests/*/`: añadir los overlays al crearlos.
- `actionlint` sobre los workflows. Se sustituye el heredado `ci-pr.yaml`; `helm-chart-ci` se borra con `helm-chart/`; los de validación por ruta (`kustomize-build-ci`, `terraform-validate-ci`, `kubevious-manifests-ci`) se mantienen o se absorben.

### Tests de punta a punta (`e2e/`) ✅ — falta engancharlos al CI

Una sola batería que entra por HTTP en `frontend` y prueba los contratos entre servicios (copias del `.proto`, traducción de errores, propagación del plazo). Escrita en k6 porque el mismo código sirve como smoke test del staging (Tarea 10) y como base de los recorridos y del usuario testigo (Tarea 12): `lib/shop.js` tiene una función por página o formulario.

| Script | Qué comprueba |
|---|---|
| `smoke.js` | Todas las páginas y formularios, errores incluidos (404, 422, 400, 403 CSRF), cabeceras de seguridad, cookie de sesión. |
| `cart-concurrent.js` | 50 usuarios con la misma sesión añaden 1 unidad a la vez y el carrito acaba con 50 (atomicidad en Redis). |
| `payment-hung.js` | `paymentservice` en pausa: 504 al agotarse el plazo de 5 s, sin detalles internos, carrito conservado. |
| `redis-down.js` | `redis-cart` parado: 503 al instante, sin detalles internos. |

`e2e/compose.yaml` usa las imágenes de producción con el endurecimiento de Kubernetes, y `run.sh` construye, levanta y ejecuta los cuatro scripts (`KEEP=1` deja la tienda en marcha). **En Kubernetes** (`e2e/kustomization.yaml`): un Job con k6 que ejecuta `smoke.js` y `cart-concurrent.js` contra el `Service` `frontend` de su namespace, con su propia `NetworkPolicy` (salida solo a `frontend:8082` y DNS) para funcionar con el deny-all, y `securityContext` restringido. `trivy config` del Job solo marca `KSV-0110` (no fija namespace, a propósito) y `KSV-0125` (k6 viene de Docker Hub; se resuelve sirviéndolo desde ACR en la Tarea 10).

---

## Tarea 7 — Infraestructura Azure (Terraform) ⏳

- VNet propia con una subred para los nodos (Azure CNI Powered by Cilium en modo *overlay*). En Azure no hay subredes «públicas» como en AWS: lo público es la IP del balanceador por el que entra el tráfico.
- **Un único clúster AKS para los 4 escenarios**, separados por namespace (NetworkPolicy, ResourceQuota y LimitRange son recursos de namespace). Dataplane **Azure CNI Powered by Cilium**. API server **público** con Entra ID/RBAC (los runners de GitHub deben poder llegar; no clúster privado).
- **3 node pools** `sistema`, `carga` y `app`, con taints, en VMs **de la serie D, no B** (los créditos de CPU de las burstable distorsionan los benchmarks). Cluster Autoscaler configurable por variable: activo en desarrollo, apagado en la campaña.
- ACR, con autenticación desde GitHub Actions por OIDC, sin credenciales estáticas.
- Estado de Terraform en Azure Blob Storage con bloqueo (bootstrap en la Tarea 5).
- **Argo CD instalado por Terraform** (`helm_release`); su *app-of-apps* instala desde git todo lo demás: kube-prometheus-stack (Prometheus, Grafana y kube-state-metrics), Kyverno, Istio, Chaos Mesh, cert-manager, el Gateway y los 4 escenarios, así el entorno reaparece solo tras cada `destroy`.
- Alertas de presupuesto (50 % y 80 %).
- **Variables:** versionar `terraform.tfvars` con los valores no secretos y quitar la regla `*.tfvars` del `.gitignore`; lo sensible, por `TF_VAR_*` u OIDC. El `terraform.tfvars` actual es el de ejemplo de GCP.
- **Entre sesiones: `terraform destroy`** (ver presupuesto).
- **Primera sesión de Azure:** comprobar que las métricas del agente de Cilium (`cilium_drop_count_total`, etc.) se pueden scrapear en el Cilium gestionado, y que el circuit breaker de Istio actúa sobre él (si no, add-on de Istio de AKS, ver Tarea 4). No se usa Hubble L7 ni ACNS (de pago).

## Tarea 8 — Escenarios de despliegue (Kustomize) ⏳

Componentes `security` y `resilience` y los 4 overlays A-D (ver sección 1); cada overlay fija su `namespace:`. La configuración común del mesh (inyección del sidecar en el namespace, reintentos a 0, recursos del proxy) va en `base` o en un componente que incluyen los 4; el circuit breaker, en `resilience`; mTLS `STRICT` y `AuthorizationPolicy`, en `security`. Las NetworkPolicy de `security` permiten el tráfico del mesh.

**Pendientes en los manifiestos** (`kustomize/base` y `kubernetes-manifests/` siguen como en el original):
- **Quitar el `loadgenerator` (Locust):** la base lo despliega y metería carga no controlada en todos los escenarios.
- **Quitar variables y endpoints sin uso:** `DISABLE_PROFILER`/`ENABLE_PROFILER` en `kubernetes-manifests/` y `kustomize/base/` de 7 servicios; `SHOPPING_ASSISTANT_SERVICE_ADDR` en `frontend`.
- **Imágenes:** usar las propias, de ACR; fijar `redis-cart` a `redis:8.10.2-alpine@sha256:3811787313eba226a2ef38658c6ccb91cd5e110edc89c37767de373120a0e5a0` (igual que `compose.yaml`); quitar `kustomize/base/**` de los `ignorePaths` de Renovate.
- **`redis-cart`** (igual en los 4): `--save ""` (sin instantáneas RDB: el `fork()` bajo carga da picos de latencia y riesgo de OOMKilled) y `--maxmemory` (p. ej. 200mb, por debajo del límite del pod) con `volatile-lru`. Sin `fork` no hace falta `vm.overcommit_memory`.
- **`REQUEST_TIMEOUT` explícito en `frontend`**, igual en los 4.
- **Requests y limits:** la base los trae en todos los Deployments; pasan al componente de seguridad (ver sección 1).
- **Límite de streams gRPC por conexión:** grpc-go y grpc-js no lo limitan por defecto, pero `frontend` usa una sola conexión por servicio, así que un límite bajo sesgaría la latencia. Si se aplica, generoso (~1000), igual en los 4 y medido antes en `kind`.
- **Memoria de la JVM de `adservice`:** el heap por defecto es el 25 % del límite. Si con carga no basta, `-XX:MaxRAMPercentage=75` vía `JAVA_OPTS` (no `JAVA_TOOL_OPTIONS`: escribe una línea que no es JSON).
- **`emailservice`:** igualar el puerto del `Service` a 8080.
- **`frontend-external` (LoadBalancer)** expone `frontend` por HTTP por fuera del Gateway: activar `components/non-public-frontend` en los 4 cuando exista el Gateway.
- Los 10 servicios ya tienen en `base` `runAsNonRoot`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false` y `drop: [ALL]`; para PSA `restricted`, el componente de seguridad añade `seccompProfile` (y lo que falte en `redis-cart`).

## Tarea 9 — Probes y autoescalado ⏳

Revisar las probes (van en el componente de resiliencia). El HPA de `frontend` es una **demo aparte**, fuera del factorial (necesita requests y variaría la medida), con el Cluster Autoscaler activo. Si el `ResourceQuota` impide al HPA crear réplicas, es consecuencia de cómo se dimensionó: se dimensiona a propósito y se documenta, no se presenta como hallazgo.

## Tarea 10 — Pipeline de CD ⏳

El clúster está destruido casi siempre, así que el CD se parte en dos:

**Siempre (al fusionar en `main`):**
1. Build y push de las 10 imágenes a ACR con tag = SHA del commit (artefacto inmutable).
2. Trivy sobre ellas. **Un único gate por SHA, igual para los 4 escenarios** (`--scanners vuln,secret --severity CRITICAL --exit-code 1`): el mismo SHA se promociona a todos.
3. Firma con `cosign` keyless (OIDC de GitHub) y SBOM.

**Solo si el clúster existe** (comprobado con `az aks show`) o por `workflow_dispatch`:
4. **Namespace de staging efímero** en el mismo clúster, con el overlay D (el más restrictivo, para ver antes lo que bloquean las políticas): despliegue, `e2e/smoke.js` y borrado del namespace, pase o no. No se usa un clúster efímero: tardaría 15-20 min por merge.
5. **Gate manual** (GitHub Environment con *required reviewer*; gratis porque el repo es público).
6. Commit que actualiza el tag en los overlays. Durante el experimento, los 4 apuntan al mismo SHA congelado (`git tag`).
7. Argo CD sincroniza.

**Rollback:** `git revert` del commit del tag.

**Además:**
- **Kyverno `verifyImages`** exige la firma de `cosign`, solo en los namespaces con seguridad ON. Comprobar que también ve la imagen del sidecar que inyecta Istio (depende del orden de los webhooks de mutación). Se prueba antes en `kind` con un registro local y una clave de prueba (la firma *keyless* solo existe dentro de GitHub Actions).
- **Imágenes de terceros** (`redis`, k6, `stress-ng`, el cliente de Redis de E3 y los proxies de Istio que corren en los namespaces de la app): se copian a ACR por digest y se firman igual, para que `verifyImages` y el registro de confianza cubran todo lo que corre en B y D.
- **Gateway API + `cert-manager`** para TLS (sesión de Azure 2), con un controlador como Envoy Gateway (ingress-nginx está retirado desde marzo de 2026), el nombre gratuito `<etiqueta>.<región>.cloudapp.azure.com` y Let's Encrypt por HTTP-01; después, `Secure` en las cookies de `frontend` y HSTS.

## Tarea 11 — Pipeline de Infra ⏳

El `plan` y los escáneres (tflint, checkov, `trivy config`) corren en cada PR, dentro del CI (Tarea 6), y bloquean igual para todo: hay un único clúster. Este pipeline se lanza a mano (`workflow_dispatch`) con dos acciones: **levantar** el entorno (`apply`) y **cerrarlo** (exportar los resultados, Tarea 14, y `destroy`). Nunca aplica solo al fusionar: crearía el clúster fuera de una sesión y gastaría presupuesto.

## Tarea 12 — Pipeline de Experimento (`workflow_dispatch`) ⏳

k6 (carga + usuario testigo, partiendo de `e2e/lib/shop.js`), Chaos Mesh (E2 y E2b) y `stress-ng` para inyectar fallos y contención de recursos, y los 4 escenarios (protocolo y variables fijadas en la sección 1). Desacoplado del CD, para repetir mediciones sin pasar por él. Las dos sesiones de campaña repiten los 4 escenarios y se redespliega el namespace entre ejecuciones. Cada ejecución es un job aparte (GitHub Actions corta los jobs a las 6 h).

**Los escenarios nunca corren en paralelo**, las métricas se filtran siempre por namespace y, con Argo CD, se evita el `selfHeal` sobre `replicas` durante la campaña (ver Tarea 13).

**Catálogo de fallos** (se construye entre las semanas 2 y 6; diez fallos diagnosticados y documentados): `OOMKilled`, `CrashLoopBackOff`, `Pending` por quota o recursos, probe mal configurada, fallo de DNS, `drain` de nodo con y sin PDB, NetworkPolicy que bloquea sin querer (incluido el tráfico del mesh), kill forzado de un pod, réplica desequilibrada con gRPC sin mesh y fallo gris (réplica `Ready` que da errores). Solo 2-3 entran en la campaña cuantitativa; el resto es análisis cualitativo (causa, señales, tiempo de recuperación). Ni `redis-cart` ni `shippingservice` son objetivo de E2, y la memoria explica por qué: matar el primero da 503 inmediatos por `FailFast` (más errores, menos latencia), y matar el segundo deja pedidos cobrados que el usuario ve fallar (ver limitaciones).

## Tarea 13 — GitOps con Argo CD ⏳

**Argo CD, no Flux** (por su interfaz web para la defensa y `ApplicationSet` para la misma app con varios overlays), instalado dentro del mismo clúster, en el pool `sistema`. Un `ApplicationSet` con generador de lista despliega los 4 overlays. Terraform sigue siendo push. Es **parte del núcleo**: el CD y el experimento dependen de él.

Durante la campaña, `ignoreDifferences` sobre `replicas` (o sincronización manual) para que el `selfHeal` no deshaga el escalado a 0 de los namespaces que no se miden. Los charts de Helm sí se usan (Argo CD, con `helm_release` desde Terraform; el resto, desde el *app-of-apps*); la carpeta `helm-chart/` no.

## Tarea 14 — Observabilidad (Prometheus + Grafana) ⏳

**Sin instrumentar el código** (ningún servicio expone `/metrics`): las métricas son idénticas en los 4 escenarios y no se añade carga a lo medido. Se pierde el detalle interno de cada servicio. Fuentes:
- **CPU, memoria y *throttling* de CPU por pod/namespace:** cAdvisor.
- **Estado, reinicios y vuelta a `Ready` (MTTR):** kube-state-metrics.
- **Paquetes bloqueados:** métricas del agente de Cilium (`cilium_drop_count_total`; sin etiqueta de namespace, así que la evidencia de H2 es el resultado binario de la prueba, no esta métrica). Sin Hubble L7: añadiría latencia solo por observar.
- **Tráfico entre servicios y circuit breaker:** telemetría de Istio (latencia y errores por servicio, expulsiones de réplicas). Recupera el detalle por servicio sin instrumentar el código, es igual en los 4 escenarios y es candidata para las métricas personalizadas.
- **Latencia y errores vistos por el usuario:** k6 (`experimental-prometheus-rw`).

**Persistencia:** dashboards como código (provisioning por ConfigMap); **antes de cada `destroy` se exportan los resultados** (k6 JSON y consultas a la API de Prometheus a CSV) y se versionan o se suben como artefacto. El análisis en Python lee esos ficheros, no Grafana.

Dashboards: MTTR y **usuario testigo** (una compra pausada y constante mientras el resto de usuarios virtuales cargan el sistema, que mide los segundos que un cliente ve la tienda caída o lenta).

**Métricas personalizadas (petición del tutor):** añadir 2-3 métricas que no vengan por defecto en kube-state-metrics, muy bien pensadas: cada una ligada a una hipótesis y obtenida sin instrumentar el código (*recording rules*, exporters o telemetría del mesh). Se deciden en esta fase, cada una con su ADR y el motivo, **como tarde en el piloto de la semana 8**: el pipeline de Experimento tiene que exportarlas desde la primera sesión de campaña.

Stretch: Loki + Alloy para recoger los logs JSON.

---

## Limitaciones a mencionar en la memoria

- **Cobro sin compensación:** `PlaceOrder` cobra y después envía. Si el envío falla, el cobro no se deshace (haría falta una *saga*; `paymentservice` no tiene reembolsos). Es el diseño original.
- **Latencia del mesh:** los proxies añaden latencia en los 4 escenarios por igual. No sesga las comparaciones, pero los valores absolutos la incluyen; se cuantifica en el piloto.
- **Circuit breaker solo en llamadas síncronas:** con colas de mensajes la resiliencia sería otra (broker, reintentos del consumidor); queda como trabajo futuro, igual que la versión HTTP de la tienda.
- **Sin gestión de secretos:** la app no tiene secretos.
- **Redis es un SPOF** en los 4 escenarios (1 réplica, sin replicación).
- **Aislamiento blando:** namespaces con NetworkPolicy son multi-tenancy blanda, no aislamiento fuerte (nodos compartidos).
- **Sin OpenTelemetry ni trazas:** decisión de alcance para no alterar lo medido.
- **Un único clúster y un único proveedor:** los resultados dependen del hardware y del Cilium gestionado de AKS.

## Stretch y opcionales

- **SLOs y alertas** (el primero si sobra tiempo; 2-3 días): 1-2 SLOs (p. ej. compras completadas por el usuario testigo y p95), reglas de Alertmanager por *burn rate* y un runbook corto por alerta. E2 y E2b deberían dispararlas.
- Gestión de secretos con External Secrets Operator y Azure Key Vault (la app no tiene secretos: con uno de demostración).
- Operación del día a día: actualización de versión de AKS sin cortar el servicio y copia y restauración con Velero.
- Argo Rollouts (despliegue canary con análisis de métricas), mejor después del TFM.
- Loki + Alloy; Tetragon.
- Mini-operador con Kubebuilder (un CRD que declare un escenario del experimento): el mejor ejercicio para entender controllers.
- Preparar CKA/CKAD/CKS (el temario encaja; el examen cuesta ~400 $, aparte del presupuesto de Azure).
- Postgres gestionado para `productcatalogservice` (solo demostrativo).

## Descartado (no reconsiderar sin motivo nuevo)

- Entornos efímeros por PR (el staging de la Tarea 10 cubre lo mismo) y clúster efímero por merge.
- Releases formales: basta con `git tag` al congelar el SHA de la campaña.
- Flux, o Flux además de Argo CD.
- Copiar `src/` a un repo nuevo.
- Guardar el carrito en una base de datos: no hay cuentas de usuario, así que Redis con caducidad es el diseño correcto.
- Testcontainers para `cartservice` (lo cubren `cart-concurrent.js` y `redis-down.js`).
- Hubble L7, ACNS y OpenTelemetry. (El mesh también añade latencia, pero es objeto de estudio y está igual en los 4 escenarios; Hubble L7 solo observaría.)
- Mesh como tercer factor (2×2×2) o solo en C y D (ver sección 1). Linkerd (versiones estables de pago), el mesh de Cilium (en AKS gestionado no se configuran circuit breakers) y el add-on de Istio de AKS como primera opción (`kind` y AKS deben ser idénticos; queda de reserva, Tarea 4). Istio ambient: el add-on de AKS no lo soporta y sobre el Cilium gestionado exige ajustes que quizá no se puedan tocar.
- Tres proyectos (gRPC, HTTP y colas): se hace solo Online Boutique en gRPC. Si el tutor lo sigue pidiendo, se migra a HTTP después de la campaña y se repite (el pipeline de Experimento lo hace barato), lo que da una comparación de protocolos con la misma lógica de negocio.
- k6 Operator (basta un Job) y `vm.overcommit_memory` (sin `fork` no hace falta).
- ingress-nginx: retirado desde marzo de 2026; se usa Gateway API.

## Verificación continua

- Tras cada fase, `e2e/run.sh` en verde y el overlay correspondiente desplegando los 10 servicios.
- Ningún workflow ni documento debe referenciar ficheros borrados.
