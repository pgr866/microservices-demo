# TFM — Memoria de contexto y hoja de ruta

> Documento único de contexto para retomar el trabajo. Recoge las decisiones vigentes y su motivo, para no volver a discutirlas sin un motivo nuevo. El detalle histórico de cada cambio está en el historial de git.

## 0. Resumen del proyecto

**Título:** Evaluación de resiliencia y seguridad en orquestación de microservicios — Un análisis basado en métricas de rendimiento y aislamiento en Kubernetes.

**Proyecto software:** fork de [Online Boutique de Google](https://github.com/GoogleCloudPlatform/microservices-demo): 10 microservicios en 5 lenguajes (el `loadgenerator` original se sustituye por k6).

**Experimento central:** test de carga con k6 e inyección de fallos en dos escenarios desplegados por los pipelines:
- **Escenario A (Estándar):** sin límites de recursos ni políticas de red.
- **Escenario B (Blindado):** con todas las restricciones de seguridad y resiliencia.

**Resultado esperado:** comparar en Grafana el coste en consumo/latencia de la seguridad frente a la velocidad de recuperación (MTTR).

**Stack:** Azure (AKS), Terraform, GitHub Actions, Argo CD, Kubernetes, Prometheus, Grafana, k6.

## Estado actual

- ✅ Tareas 1-3: fork limpio y los 10 microservicios validados en local (tests, linter, formato, Dockerfiles endurecidos, entorno de desarrollo).
- ✅ Tests de punta a punta en `e2e/` (adelanta parte de las Tareas 6, 10 y 12).
- ⏳ Tareas 4-14: diseñadas, sin implementar.
- **Rama de trabajo:** `refactor/local-microservice-validation`; `main` no se toca hasta fusionar por PR.
- **Orden:** primero todo lo que no cuesta dinero (Tareas 4-6: `kind`, Terraform sin `apply`, CI). El primer `terraform apply` contra Azure (Tareas 10-11) va al final.

---

## Tarea 1 — Fork incremental ✅

Se modifica este fork en vez de copiar `src/` a un repo nuevo: los manifiestos, `kustomize/base` y `kustomize/components/network-policies` (el aislamiento del Escenario B) ya existen, y el historial de git es evidencia citable del proceso.

## Tarea 2 — Limpieza del fork ✅

**Borrado:** todo lo exclusivo de Google Cloud o del proyecto original sin valor aquí — `.deploystack/`, `cloudbuild.yaml`, `skaffold.yaml`, `istio-manifests/`, `release/` y `docs/releasing/`, los workflows atados a su proyecto GCP (`ci-main`, `deploy-pr`, `cleanup`, `make-release`) con `.github/terraform/` y `.github/release-cluster/`, la gobernanza de proyecto open source (CODEOWNERS, plantillas, etc.) y `shoppingassistantservice` (fuera del alcance de 10 servicios).

**Se mantiene aunque no se use directamente:**
- `terraform/` (GCP): referencia de estructura hasta escribir el de Azure (Tarea 7).
- `helm-chart/`: no se usa (el despliegue es Kustomize + Argo CD), solo referencia.
- Componentes opcionales de `kustomize/components/`: ejemplos de cómo componer overlays. `google-cloud-operations` está obsoleto (activa Profiler/Tracing que el código ya no tiene).
- `src/loadgenerator/` (Locust): referencia de los recorridos de usuario; borrar cuando exista el script de carga de k6 (Tarea 12).
- Workflows `kustomize-build-ci`, `terraform-validate-ci`, `kubevious-manifests-ci` y `helm-chart-ci` (validan por ruta, siguen sirviendo), y `ci-pr.yaml` (CI heredado, a sustituir en la Tarea 6).
- `.github/renovate.json5`: actualiza las dependencias de los servicios. Ignora `kustomize/base/**` hasta la Tarea 8.

---

## Tarea 3 — Validación local de microservicios ✅

### Reglas (se aplican a cualquier cambio en un servicio)

1. Compila, arranca y pasa sus tests.
2. Sin código GCP-específico (Profiler, metadata server, AlloyDB/Spanner…) ni OpenTelemetry: no hay Collector; la observabilidad es la de la Tarea 14.
3. Sin código muerto: funciones sin uso, ramas inalcanzables o equivalentes, variables de entorno que nadie lee, ficheros que no compilan.
4. **Tests solo de lógica real y alcanzable sin red** (parseo, cálculo, validación, traducción de errores, plantillas). Sin servidor gRPC real; las dependencias se sustituyen por fakes en memoria. Lo que exige abrir un puerto (arranque del servidor) queda sin cubrir a propósito. Las pruebas con los servicios reales van en `e2e/`.
5. Linter y formateador con la misma herramienta que el CI, configurados dentro de cada servicio (excepción: `.editorconfig` en la raíz, estándar por repositorio).
6. **Dockerfile de producción:** base fijada por digest; imagen final mínima sin herramientas de build; usuario no-root explícito; ningún `COPY` al stage final con `--chown` (los ficheros quedan de root y el proceso no puede modificar su propio código aunque falte `readOnlyRootFilesystem`).
7. **`Dockerfile.dev` multietapa:** cada comando del README es una etapa con el mismo nombre en los 10 servicios — `test`, `coverage`, `lint`, `format`, `format-check`, `dependencies`, `codegen` y, en Go, `vulncheck` (`cartservice` no tiene `format-check`, porque su `lint` ya comprueba el formato, ni `codegen`, y su `dependencies` solo informa: NuGet no tiene un comando que reescriba las versiones del `.csproj`). Se ejecutan con `docker build -f Dockerfile.dev --target <etapa> .`, y lo que generan sale con `--output type=local,dest=.` (a nombre del usuario, no de root). La última etapa, `dev`, es el hot-reload que construye Compose. Igual en local que en CI: nada de `setup-<lenguaje>`.
8. Código generado (`genproto/`, `demo_pb2*.py`, `obj/`, `build/generated`) fuera del linter y de la cobertura. Nunca se tocan las cabeceras de licencia.

### Estado y herramientas por servicio

| Servicio | Lenguaje | Tests | Linter | Hot-reload | Umbral de cobertura |
|---|---|---|---|---|---|
| `productcatalogservice` | Go | `go test` | golangci-lint | `air` | 45% |
| `shippingservice` | Go | `go test` | golangci-lint | `air` | 55% |
| `checkoutservice` | Go | `go test` | golangci-lint | `air` | 70% |
| `frontend` | Go | `go test` | golangci-lint | `air` | 70% |
| `paymentservice` | Node | Jest | ESLint + Prettier | `node --watch` | 70% |
| `currencyservice` | Node | Jest | ESLint + Prettier | `node --watch` | 70% |
| `emailservice` | Python | pytest | ruff | `watchmedo` | 70% |
| `recommendationservice` | Python | pytest | ruff | `watchmedo` | 70% |
| `adservice` | Java | JUnit 6 + JaCoCo | Checkstyle + google-java-format | `watchexec --poll` | 30% |
| `cartservice` | C# | xunit v3 + Coverlet | `dotnet format` | `dotnet watch` | 70% |

Los umbrales por debajo del 70% general son de servicios donde casi todo lo no cubierto es arranque del servidor (regla 4); se prefiere bajar el umbral a excluir código propio del cómputo.

**Hallazgo intencionado del linter:** cada servicio deja a propósito **un único** hallazgo, siempre una expresión booleana redundante, como demostración de que el linter funciona en CI. No es un descuido; no corregirlo.

| Stack | Regla | Dónde |
|---|---|---|
| Go | `S1002` (staticcheck) | lectura de `PORT` en `main` (`== true`) |
| Java | `SimplifyBooleanExpression` | `blockUntilShutdown()` |
| Node | `no-extra-boolean-cast` | `charge.js` (payment), guard `require.main` (currency) |
| Python | `SIM210` | `logger.py` (email), `start()` (recommendation) |
| C# | `WHITESPACE` (`dotnet format`) | `RequireUserId(...)` de `EmptyCart`; el comando de formato del README también lo corrige |

### Decisiones vigentes

**Puertos:** cada servicio usa el mismo puerto en desarrollo y en producción, sin remapear en Compose: `paymentservice` 50052, `recommendationservice` 8081 y `frontend` 8082 (cambiados respecto al original para no chocar), `shippingservice` 50051, `emailservice` 8080. En los manifiestos, el `Service` de `emailservice` sigue publicando el puerto 5000 heredado (ver Tarea 8).

**Códigos de error gRPC** (guía de gRPC y AIP-193), igual en todos los servicios:
- fallo de la petición → `InvalidArgument`;
- estado del sistema (p. ej. carrito vacío, producto inexistente en el carrito) → `FailedPrecondition`;
- dependencia caída o saturada → `Unavailable`;
- cualquier otro fallo → `Internal`.
- excepciones estándar: `NotFound` en `productcatalogservice` (producto inexistente, AIP-193), y `DeadlineExceeded`/`Canceled` en `checkoutservice` solo cuando vence o se cancela el contexto del propio llamante.

El código de una dependencia nunca se reenvía tal cual y nunca viaja una traza al cliente. `frontend` traduce a HTTP con la tabla de `google/rpc/code.proto`. En un 4xx muestra el motivo; en un 5xx, un mensaje genérico y el *request ID*, que enlaza con la línea de log completa.

**Plazos:**
- `frontend` corta cada petición a los `REQUEST_TIMEOUT` (por defecto `5s`), y el plazo se propaga por toda la cadena.
- Sin reintentos automáticos en ningún cliente (ver Tarea 12).
- `cartservice` falla al instante si Redis no responde (`BacklogPolicy.FailFast`) en vez de esperar al plazo.

**Logs:** JSON con `timestamp` (ISO 8601 UTC), `severity` y `message`, más el nombre del logger.
- `LOG_LEVEL` = `debug`, `info` (por defecto), `warn` o `error`; un valor desconocido hace fallar el arranque con una línea JSON.
- `info` para arranque, configuración, apagado y cobros; lo que se repite en cada petición, a `debug` (`compose.yaml` usa `debug`).
- **Nunca** en los logs: número de tarjeta ni CVV (PCI DSS), email del cliente ni ID de sesión (es la clave del carrito). Hay tests de regresión de esto.

**Apagado ordenado con SIGTERM** en los 10 servicios: dejan de aceptar peticiones, el health check pasa a `NOT_SERVING`, terminan las que tienen en curso en un plazo de 10 s (por debajo de los 30 s de Kubernetes) y después cortan el resto. En Node y Python hace falta el manejador explícito: como PID 1 del contenedor, sin él ignorarían SIGTERM.

**`cartservice` / Redis:**
- Cada carrito es un hash `cart:<usuario>` con un campo por producto.
- `HINCRBY` + `EXPIRE` de 48 h (la vida de la cookie de sesión) en una transacción: es atómico también con varias réplicas, y ningún carrito queda sin caducidad.
- El health check responde `SERVING` sin depender de Redis, para que un Redis caído no reinicie todos los pods; solo pasa a `NOT_SERVING` durante el apagado, como el resto.
- Sin `REDIS_ADDR` usa un almacén en memoria.
- Redis sin contraseña, a propósito: limitar quién llega a él es justo el eje A vs B (`NetworkPolicy` de `redis-cart`).

**`frontend` (expuesto a internet):**
- CSP estricta, sin nada en línea; cabeceras `X-Content-Type-Options`, `X-Frame-Options` y `Referrer-Policy`.
- CSRF con `http.NewCrossOriginProtection()`; las peticiones sin cabeceras de navegador (k6, curl) pasan.
- Cookies `HttpOnly` y `SameSite=Lax`; `Secure` y HSTS cuando haya TLS (Tarea 10).
- Timeouts del servidor HTTP y límite de 64 KiB al cuerpo de las peticiones.
- La home no llama a `adservice` (no tiene hueco de anuncio).

**Imágenes base y herramientas** (todas fijadas; las de producción, por digest):
- **Lenguajes:** `golang:1.27.2-alpine`, `node:24.21.0-alpine`, `python:3.14.8-alpine`, `eclipse-temurin:25` (JDK, solo en el builder) y .NET 10 (SDK `10.0.401-alpine3.24`).
- **Bases de la imagen final:** distroless `static:nonroot` en Go, `alpine:3.24.2` en Node y Java, y `runtime-deps:10.0.12-alpine3.24` en C#.
- **Herramientas:** `redis:8.10.2-alpine` (por digest también en `compose.yaml`), `golangci-lint v2.14.0`, `grpcurl v1.9.3`, `govulncheck v1.8.0`, `uv 0.12.23`, Trivy `0.76.0` y k6 `2.3.0`.
- `adservice` lleva un JRE propio generado con `jlink` sobre `alpine`.
- Python copia solo `site-packages` a la imagen final y desinstala `pip`.
- Node se queda en la 24 (LTS) y Python en la 3.14 hasta que salgan versiones estables nuevas.

**Dependencias:**
- Sin `overrides` en `package.json` para silenciar avisos de dependencias transitivas de desarrollo. Avisos conocidos que se dejan:
  - `glob@10.5.0` y `sprintf-js` dentro de Jest;
  - `Configuration.setVisible` del plugin de protobuf de Gradle 0.10.0, que ya es su última versión.
- npm deniega los scripts de instalación (`allowScripts`), y producción se instala con `npm ci --omit=dev`.
- Las dependencias de test van en un fichero aparte de las de producción.

**Escaneo de vulnerabilidades:**
- Trivy (`--scanners vuln,secret`) sobre las 10 imágenes, y `govulncheck` en los 4 servicios Go, porque detecta vulnerabilidades alcanzables que Trivy no marca.
- `trivy config` de los Dockerfile de producción: solo `DS-0026` (sin `HEALTHCHECK`), aceptado porque Kubernetes usa sus probes. Los `Dockerfile.dev` dan `DS-0002` (root en las etapas de herramientas); son solo de desarrollo.
- **Pendiente externo:** `zlib` CVE-2026-85091 (MEDIUM) en las 6 imágenes Alpine. Se espera a que se republiquen las imágenes base, sin forzarlo con `apk upgrade`.

**Entorno de desarrollo:**
- `compose.yaml` levanta los 10 servicios más `redis-cart` con hot-reload.
  - Cada servicio tiene su `healthcheck`, contra `127.0.0.1` y no `localhost`, porque BusyBox resuelve `localhost` solo a IPv6.
  - `depends_on` usa `service_healthy`.
  - `redis-cart` lleva el mismo endurecimiento que en Kubernetes.
- **`WORKDIR /app` en todos.**
- La etapa `dev` corre con un usuario de UID 1000, para que lo que se escriba en las carpetas montadas sea del desarrollador. Las cachés van en volúmenes con nombre por servicio.
  - Tras crear un volumen como root, hay que borrarlo una vez con `docker compose down -v`.
  - Tras cambiar `package.json`: `docker compose down -v <servicio>` y luego `up --build`.
- **Devcontainers:** uno por servicio en `.devcontainer/<servicio>/`, que reutiliza el servicio de Compose; git por HTTPS con la sesión de GitHub de VS Code.
  - Si el servicio ya estaba en marcha, hay que abrirlo con *Rebuild Container*.
  - Mientras no salga [docker/compose#14286](https://github.com/docker/compose/pull/14286), construirlo exige `BUILDX_BAKE_ENTITLEMENTS_FS=0` en el host.
  - En Windows, el repo se clona dentro de WSL2 (desde `C:\` no hay hot-reload).
- `adservice`: parar su contenedor de desarrollo antes de lanzar las etapas de Gradle, porque comparten el bloqueo de `.gradle/`.

---

## Tarea 4 — Validación local de Kubernetes (`kind`) ⏳

- Clúster `kind` con **Cilium** como CNI (kindnet no aplica NetworkPolicies), igual que el dataplane de AKS (Tarea 7).
- Las imágenes de `kustomize/base` siguen siendo las publicadas por Google, sin los cambios de la Tarea 3. Hay que construir las propias y cargarlas con `kind load docker-image`, usando el componente `container-images-registry`/`tag` o el overlay propio.
- **Escenario A:** los 10 servicios y `redis-cart` sanos, con las probes en verde.
- **Escenario B:** las NetworkPolicies bloquean el tráfico no autorizado (p. ej. acceso a Redis desde un pod sin permiso) y dejan pasar el legítimo. Comprobar también que un pod que excede `LimitRange`/`ResourceQuota` es rechazado.
- **Comprobar que Hubble expone `cilium_drop_count_total`** y las métricas L7. La Tarea 14 depende de ellas.
- Ejecutar `e2e/smoke.js` contra los dos namespaces (k6 como Job apuntando a `frontend.<namespace>.svc`), y una prueba de humo de la inyección de fallos y de la carga.
- Medir con carga, antes de decidir, los pendientes de la Tarea 8: memoria de la JVM de `adservice` y límite de streams gRPC.
- (Opcional) Argo CD en el mismo `kind`, para validar el `ApplicationSet`.

## Tarea 5 — Validación local de Terraform ⏳

`terraform fmt -check`, `validate` y `plan` (solo necesita credenciales, no crea nada). Solo con las Tareas 3, 4 y 5 en verde se ejecuta el primer `terraform apply`.

---

## Tarea 6 — Pipeline de CI ⏳

No depende de Azure, salvo las credenciales de solo lectura que necesita `terraform plan` (OIDC). Va creciendo de alcance a medida que las Tareas 7-9 añaden contenido. Se dispara en cada PR, con dos tracks en paralelo. **Todo se ejecuta en Docker, con las etapas de `Dockerfile.dev`**: cada etapa genera el formato que espera la action de informe correspondiente, y esa action anota la PR.

### A) Matriz por microservicio

1. **Tests + cobertura**, en una sola ejecución (etapas `test`/`coverage`).
   - Informe de tests con `dorny/test-reporter`: `golang-json`, `java-junit` (sirve también para el JUnit de pytest y de `jest-junit`) y `dotnet-trx`.
2. **Gate de cobertura, bloqueante**, con los umbrales de la tabla de la Tarea 3:
   - Go → `gwatts/go-coverage-action`;
   - Java → `madrapps/jacoco-report`;
   - Node → `ArtiomTr/jest-coverage-report-action`;
   - Python y .NET → Cobertura XML con `irongut/CodeCoverageSummary`.

   Gradle + JaCoCo es la combinación más frágil: si falla, se documenta como limitación.
3. **Linter, informativo (no bloquea)**, con anotaciones en la PR:
   - `golangci-lint-action`, `reviewdog/action-eslint` y `astral-sh/ruff-action`;
   - reviewdog con `-f=dotnet` y `-f=checkstyle`. Antes de pasarle la salida hay que quitar el prefijo `/app/` de las rutas (`sed 's|/app/||g'`), porque `dotnet format` y Checkstyle guardan rutas del contenedor.
4. **Formato, informativo — decisión del usuario, no reconsiderar.** Usa la etapa `format-check`, y el CI nunca formatea. En `cartservice`, el `lint` ya cubre el formato.
5. **`go test -race`** en los 4 Go, en un paso propio (necesita CGO y `gcc`, que no tienen las etapas): la carrera de datos que se corrigió en `productcatalogservice` solo se detecta así.
6. **Caché de BuildKit:** los Dockerfile usan `--mount=type=cache`, pero GitHub Actions no la conserva entre ejecuciones. Hay que persistirla (p. ej. `buildkit-cache-dance` con `actions/cache`) o no se gana nada en CI.

### B) Infraestructura, imágenes y manifiestos

- `docker build` de las 10 imágenes de producción + Trivy (informativo; el bloqueo está en el CD) + `govulncheck`.
- **Tests de punta a punta (`e2e/run.sh`), job bloqueante** (ver abajo).
- `terraform fmt`/`validate`/`plan` comentado en la PR, + tfsec, checkov y tflint (informativos).
- `kustomize build` de los overlays A y B + `kube-linter`, que además da un dato comparativo entre A y B. `kustomize-build-ci` hoy solo compila `kustomize/` y `kustomize/tests/*/`: añadir los overlays al crearlos.
- `actionlint` sobre los workflows.

### Tests de punta a punta (`e2e/`) ✅ — falta engancharlos al CI

Los tests unitarios usan fakes (regla 4), así que los contratos entre servicios (copias del `.proto`, traducción de errores, propagación del plazo) y el recorrido completo se prueban aquí. Es **una sola batería que entra por HTTP en `frontend`**, no tests por pareja de servicios. Se escribe con **k6** porque el mismo código sirve después como smoke test del staging (Tarea 10) y como base de los recorridos y del usuario testigo (Tarea 12): por eso `lib/shop.js` tiene una función por página o formulario.

**`compose.yaml`** usa las **imágenes de producción** con el endurecimiento de Kubernetes (`read_only`, `cap_drop: ALL`, `no-new-privileges`), en un proyecto propio sin puertos en el host.
- **`ready`** (contenedor de un solo uso con `grpcurl`) espera hasta 120 s a que los 9 servicios gRPC respondan a `Health/Check`, `frontend` a `/_healthz` y `redis-cart` en su puerto, igual que las *readiness probes*. Si alguno no responde, el log dice cuál.
  - Va en un contenedor aparte porque las imágenes distroless no tienen shell para un `healthcheck`.
  - `init: true` es imprescindible: sin él, `sh` queda como PID 1 e ignora el SIGTERM de `timeout`.
- k6 depende de `ready` y está en el perfil `test`.

| Script | Qué comprueba |
|---|---|
| `smoke.js` | Todas las páginas y formularios, errores incluidos (404, 422, 400, 403 CSRF), cabeceras de seguridad, cookie de sesión y que su ID no salga en el HTML. El anuncio se da por bueno si aparece en alguno de 5 intentos, porque es opcional por diseño. |
| `cart-concurrent.js` | 50 usuarios con la misma sesión añaden 1 unidad a la vez y el carrito acaba con 50 (atomicidad en Redis). |
| `payment-hung.js` | `paymentservice` en pausa (acepta la llamada y no contesta): 504 al agotarse el plazo de 5 s, sin detalles internos, y el carrito se conserva. |
| `redis-down.js` | `redis-cart` parado: 503 al instante, sin detalles internos. |

Umbrales: `checks: rate==1`, y en los dos primeros también `http_req_failed: rate==0` (un 5xx en cualquier petición falla el test).

`run.sh` construye, levanta y ejecuta los cuatro scripts en ese orden; los dos últimos con `--no-deps`, porque `ready` esperaría al servicio caído. Al terminar restaura los servicios y lo borra todo con `down --profile test`; `KEEP=1` deja la tienda en marcha. Sale con error si falla cualquier comprobación.

**Descartado:** integración de `cartservice` con Redis mediante Testcontainers. Los tests corren dentro de `docker build`, sin acceso a Docker, y lo importante (atomicidad y Redis caído) ya lo cubren `cart-concurrent.js` y `redis-down.js`.

---

## Tarea 7 — Infraestructura Azure (Terraform) ⏳

- VNet segmentada: subred pública para el Ingress y subred privada para los nodos.
- **Un único clúster AKS para A y B**, separados por namespace. NetworkPolicy, ResourceQuota y LimitRange son recursos de namespace, así que se consigue el mismo aislamiento con la mitad de nodos.
  - Dataplane **Azure CNI Powered by Cilium**, por Hubble y su `cilium_drop_count_total` por namespace (el Azure NPM clásico no da ese dato).
  - Cluster Autoscaler con mínimo y máximo de nodos.
- ACR, con autenticación desde GitHub Actions por OIDC, sin credenciales estáticas.
- Estado de Terraform en Azure Blob Storage, con bloqueo.
- **`vm.overcommit_memory = 1` en el node pool** (`linux_os_config.sysctl_config`): lo pide `redis-cart` y no se puede fijar desde el pod.
- **Variables:** versionar `terraform.tfvars` con los valores no secretos (región, tamaño de nodos…) y quitar la regla `*.tfvars` del `.gitignore`; lo sensible, por `TF_VAR_*` u OIDC en el pipeline. El `terraform.tfvars` actual es el de ejemplo de GCP.
- **Entre sesiones de trabajo, `az aks stop`/`az aks start`**, para no pagar cómputo mientras no se usa.

## Tarea 8 — Dos escenarios de despliegue (Kustomize) ⏳

- **A** = `kustomize/base` en el namespace `boutique-baseline`.
- **B** = overlay propio en `boutique-hardened`, con `components/network-policies`, `ResourceQuota` y `LimitRange` (estos dos, por crear).
- Cada overlay fija su `namespace:`.

**Pendientes en los manifiestos:**
- **Variables y endpoints sin uso:**
  - `DISABLE_PROFILER`/`ENABLE_PROFILER` aparecen en `kubernetes-manifests/` y `kustomize/base/` de 7 servicios.
  - `SHOPPING_ASSISTANT_SERVICE_ADDR` en `frontend`.

  Nada de esto lo lee el código. Quitarlas.
- **Imágenes:**
  - Usar las propias, de ACR, en lugar de las de Google.
  - `redis-cart` usa `redis:alpine` sin versión: fijar `redis:8.10.2-alpine@sha256:3811787313eba226a2ef38658c6ccb91cd5e110edc89c37767de373120a0e5a0`, la misma que `compose.yaml`.
  - Quitar `kustomize/base/**` de los `ignorePaths` de Renovate.
- **`redis-cart`:**
  - Desactivar las instantáneas RDB (`--save ""`): bajo carga, cada copia hace `fork()`, con picos de latencia y riesgo de OOMKilled, y solo salvan un reinicio del contenedor.
  - Fijar `--maxmemory` (p. ej. 200mb) por debajo del límite del pod, con `volatile-lru`: así Redis expulsa carritos en vez de que lo mate Kubernetes.
  - Igual en A y en B.
- **`REQUEST_TIMEOUT` explícito en `frontend`**, igual en A y en B (ver Tarea 12).
- **Endurecimiento solo en B** (A es la línea base con la que se compara):
  - `seccompProfile: RuntimeDefault` (lo exige el estándar *restricted* de los Pod Security Standards).
  - `automountServiceAccountToken: false` (ningún servicio usa la API de Kubernetes).

  Los 10 servicios ya tienen en `base` `runAsNonRoot`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false` y `drop: [ALL]`.
- **Límite de streams gRPC simultáneos por conexión:** grpc-go y grpc-js no ponen límite por defecto. Pero `frontend` usa una sola conexión por servicio, así que un límite bajo sesgaría la latencia. Si se aplica, que sea generoso (~1000), igual en A y en B, y medido antes en `kind`.
- **Memoria de la JVM de `adservice`:** por defecto, el *heap* es el 25% del límite del contenedor. Si con carga en `kind` no basta, poner `-XX:MaxRAMPercentage=75` vía `JAVA_OPTS` (no `JAVA_TOOL_OPTIONS`: escribe una línea que no es JSON).
- **`preStop` con una espera corta** (p. ej. `sleep 5`), más un `terminationGracePeriodSeconds` que cubra esa espera y los 10 s del apagado. El endpoint de un pod que se borra se retira de forma asíncrona.
- **`frontend-external` (LoadBalancer)** expone `frontend` por HTTP por fuera del Ingress. Activar `components/non-public-frontend` en A y en B cuando exista el Ingress.
- **`emailservice`:** el `Service` publica el puerto 5000 (`targetPort` 8080), heredado. Decidir si se iguala a 8080.

## Tarea 9 — Probes y autoescalado ⏳

Revisar las probes y añadir un HPA en `frontend`. En B, el `ResourceQuota` puede impedir que el HPA cree réplicas: no es un bug, es un dato comparativo (el blindaje limita la reacción del autoescalado).

## Tarea 10 — Pipeline de CD ⏳

Al fusionar en `main`:
1. Build y push de las 10 imágenes a ACR, con tag = SHA del commit (artefacto inmutable), y Trivy sobre ellas.
2. **Namespace de staging efímero** en el mismo clúster: despliegue, `e2e/smoke.js` y borrado del namespace, pase o no. No se usa un clúster efímero: tardaría 15-20 min por cada merge.
3. **Gate manual** (GitHub Environment con *required reviewer*; gratis porque el repo es público).
4. Commit que actualiza el tag en el overlay correspondiente:
   - **La promoción a B se bloquea si Trivy encuentra algo CRITICAL** (`--scanners vuln,secret --severity CRITICAL --exit-code 1`).
   - La de A solo informa, y el dato queda para la comparativa.
5. Argo CD sincroniza.

**Rollback:** `git revert` del commit del tag.

**Además:**
- Ingress Controller (nginx o AGIC) + `cert-manager` para TLS; después, `Secure` en las cookies de `frontend` y HSTS.
- **Opcional:** firma con `cosign` + SBOM. Solo tiene sentido si también se verifica la firma (Kyverno `verifyImages` o `cosign verify`), con el mismo criterio de A informa / B bloquea.

## Tarea 11 — Pipeline de Infra ⏳

Se dispara con los cambios en `terraform/` y aplica la Tarea 7. tfsec/checkov **bloquea solo en B**; en A informa, y ese dato es parte del eje comparativo del TFM.

## Tarea 12 — Pipeline de Experimento (`workflow_dispatch`) ⏳

k6 (carga + usuario testigo, partiendo de `e2e/lib/shop.js`) y un script de inyección de fallos con `kubectl`, contra A y luego contra B. Desacoplado del CD, para repetir mediciones sin redesplegar.

- **k6 corre dentro del clúster** contra el `ClusterIP` de `frontend` (`frontend.<namespace>.svc.cluster.local`), nunca a través del Ingress: medir pasando por Ingress, DNS y TLS añadiría latencia de esas capas.
- **Protocolo:**
  - A y B nunca en paralelo.
  - Entre una ejecución y otra, esperar a que el autoscaler vuelva al mínimo de nodos.
  - Métricas siempre filtradas por namespace.
- **Fijado antes de medir e igual en A y B:**
  - **`REQUEST_TIMEOUT`.**
  - **`MAX_WORKERS`** de los servicios Python (por defecto 50 hilos y hasta 2×N llamadas en curso; las que sobran reciben `RESOURCE_EXHAUSTED` al instante).
  - **Llamadas en paralelo por producto** en `frontend` y `checkoutservice`, con un máximo de 10 a la vez (`errgroup`): la latencia ya no es la suma de todas, a diferencia del Online Boutique original.
  - **Sin reintentos automáticos:** ocultarían parte de los fallos inyectados.
  - **Tipo de fallo:**
    - `kubectl delete pod` es un fallo «educado»: con el apagado ordenado, el pod termina antes sus peticiones en curso.
    - `--grace-period=0 --force` simula una caída real.
    - Son dos experimentos distintos; citar en la memoria el que se use.
  - **Microservicio objetivo fijo.**
- **Si el objetivo es `redis-cart`:** con `FailFast`, un corte breve se ve como 503 inmediatos (más errores, menos latencia). Citarlo en la memoria.
- **Si el objetivo es `shippingservice`:** habrá pedidos cobrados que el usuario ve fallar (ver limitaciones).
- **N ≥ 3 repeticiones por escenario**: en Grafana, media/mediana y desviación, no un valor único.

## Tarea 13 — GitOps con Argo CD ⏳

**Argo CD, no Flux** (por su interfaz web para la defensa y `ApplicationSet` para la misma app con dos overlays), instalado dentro del mismo clúster. Un `ApplicationSet` despliega `base` en `boutique-baseline` y el overlay blindado en `boutique-hardened`. Terraform sigue siendo push. El CLI `helm` sí se usa (charts de `kube-prometheus-stack` y de Argo CD); la carpeta `helm-chart/` no. **Es la primera pieza recortable si falta tiempo**: no afecta a ningún resultado.

## Tarea 14 — Observabilidad (Prometheus + Grafana) ⏳

**Sin instrumentar el código** (ningún servicio expone `/metrics`). Es una decisión de alcance: así las métricas son idénticas en A y B y no se añade carga a lo que se mide. Se pierde el detalle interno de cada servicio. Fuentes:
- **CPU y memoria por pod/namespace:** cAdvisor.
- **Estado, reinicios y vuelta a `Ready` (base del MTTR):** kube-state-metrics.
- **Paquetes bloqueados (`cilium_drop_count_total`) y tráfico entre servicios:** Hubble.
- **Método, código y latencia entre servicios:** Hubble L7. No viene activado y añade algo de latencia por el proxy: activarlo igual en A y en B.
- **Latencia y errores vistos por el usuario:** k6 (`experimental-prometheus-rw`).

Dashboards: MTTR y **usuario testigo** (una compra pausada y constante mientras el resto de usuarios virtuales cargan el sistema, que mide los segundos que un cliente ve la tienda caída o lenta).

---

## Limitaciones a mencionar en la memoria

- **Cobro sin compensación:** `PlaceOrder` cobra y después envía. Si el envío falla, el cobro no se deshace: haría falta una *saga*, y `paymentservice` no tiene reembolsos. Es el diseño original.
- **Tráfico interno sin cifrar (sin mTLS):** el eje medido es el aislamiento de red, no el cifrado. Hacerlo bien exigiría un *service mesh*, desproporcionado para este TFM.
- **Sin gestión de secretos:** la app no tiene secretos. Si se añadiera Postgres, la cadena de conexión vendría de Key Vault (CSI driver).

## Mejoras opcionales

- Postgres gestionado para `productcatalogservice` (es de solo lectura). Puramente demostrativo.

## Descartado (no reconsiderar sin motivo nuevo)

- Entornos efímeros por PR: el staging de la Tarea 10 cubre lo mismo.
- Releases formales (semantic-release): basta con `git tag` al congelar cada escenario.
- Flux, o Flux además de Argo CD.
- Copiar `src/` a un repo nuevo.
- Guardar el carrito en una base de datos: no hay cuentas de usuario, así que Redis con caducidad es el diseño correcto.
- Testcontainers para `cartservice` (ver `e2e/`).

## Verificación continua

- Tras cada fase, `e2e/run.sh` en verde, y el overlay correspondiente desplegando los 10 servicios.
- Ningún workflow ni documento debe referenciar ficheros borrados.
