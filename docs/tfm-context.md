# TFM — Memoria de contexto y hoja de ruta

> Documento único de contexto para retomar el trabajo en cualquier conversación futura. Organizado como lista de tareas en orden cronológico: cada una indica su estado y toda la decisión/razón detrás, para no tener que volver a discutirla sin motivo nuevo.

## 0. Resumen del proyecto

**Título:** Evaluación de resiliencia y seguridad en orquestación de microservicios — Un análisis basado en métricas de rendimiento y aislamiento en Kubernetes.

**Proyecto software:** fork de [Online Boutique de Google](https://github.com/GoogleCloudPlatform/microservices-demo) (11 microservicios).

**Experimento central:** ejecutar un test de carga con k6 y una inyección de fallos en dos escenarios desplegados automáticamente por los pipelines:
- **Escenario A (Estándar):** sin límites de recursos ni políticas de red.
- **Escenario B (Blindado):** con todas las restricciones de seguridad y resiliencia activadas.

**Resultado esperado:** comparar cuantitativamente en Grafana el equilibrio entre el consumo/latencia que introduce la seguridad aplicada frente a la velocidad de recuperación del sistema (MTTR).

**Stack:** Azure (AKS), Terraform, GitHub Actions, Argo CD, Kubernetes, Prometheus, Grafana.

## Estado actual (para retomar rápido)

- ✅ Tarea 1 — Método de trabajo: fork incremental (decidido).
- ✅ Tarea 2 — Limpieza del fork: 56 ficheros borrados, **commiteado** en `eef36dbb` ("chore: remove Google Cloud–specific tooling and out-of-scope service").
- ✅ Tarea 3 — Alcance del TFM documentado.
- ⏳ Tarea 4 — validación local de microservicios: en curso. `productcatalogservice` ya revisado a fondo (referencia para el resto, quedan 10). Incluye escribir tests para los 6 microservicios sin cobertura — la investigación de qué testear ya está hecha a fondo, el código de los tests todavía no está escrito.
- ⏳ Todo lo demás (tareas 5-15) — decidido en diseño, **nada implementado todavía**.
- **Regla de oro para el orden de ejecución real:** primero todo lo que no cuesta dinero. Eso incluye las Tareas 4-6 (validación local: microservicios, Kubernetes con `kind`, Terraform) y la Tarea 7 (pipeline de CI, que no toca Azure salvo un matiz — ver esa tarea). El primer `terraform apply` real contra Azure (Tareas 11 y 12) es deliberadamente uno de los últimos pasos, no el primero.

---

## Tarea 1 — Método de trabajo: fork incremental ✅

**Decisión:** seguir modificando este mismo fork de forma incremental. No copiar solo `src/` a un repo nuevo para reconstruir desde cero.

**Razones:**
1. El 90% del "software project" que pide el enunciado ya está aquí y es reutilizable: manifiestos Kubernetes de los 11 microservicios, `kustomize/base` + `kustomize/components/network-policies/` (exactamente el aislamiento de red del Escenario B). Reescribirlo a mano es trabajo mecánico sin valor para el TFM y con riesgo de bugs.
2. El historial de git es evidencia de proceso citable en la memoria.
3. Menor riesgo de quedarse a medias — cada pieza se prueba según se añade/borra.
4. La única razón real para reescribir sería el ejercicio de aprender a escribir YAML de Kubernetes desde cero, y eso no es lo que evalúa el TFM.

---

## Tarea 2 — Auditar y limpiar el fork ✅ (commit `eef36dbb`)

### Borrado definitivamente (56 rutas, sin ningún valor ni como ejemplo)

- **Tooling de despliegue exclusivo de Google Cloud**: `.deploystack/` (Cloud Shell, usa Secret Manager y un proyecto GCP privado), `cloudbuild.yaml` (Google Cloud Build).
- **`skaffold.yaml`** — herramienta de bucle de desarrollo local, sustituida por el pipeline de CI/CD; además referenciaba servicios ya borrados.
- **`istio-manifests/`** — duplicado literal y obsoleto de `kustomize/components/service-mesh-istio/` (que sí se conserva).
- **Documentación del tooling borrado**: `docs/deploystack.md`, `docs/cloudshell-tutorial.md`.
- **Proceso de release oficial del proyecto de Google**: `docs/releasing/` (scripts que hacen build+push a su Artifact Registry y cortan releases del repo oficial), `release/` (artefactos generados por esos scripts, duplicados de `kubernetes-manifests/`+`istio-manifests/`), `.github/workflows/make-release.yaml`.
- **CI/CD hardcodeada al proyecto GCP privado de Google (`online-boutique-ci`)**: `.github/workflows/ci-main.yaml`, `deploy-pr.yaml`, `cleanup.yaml` (mecanismo de *entorno efímero por PR*, descartado también por decisión propia — ver "Decisiones descartadas explícitamente" al final del documento), `.github/terraform/` (provisiona el clúster GKE compartido que usaban esos workflows), `.github/release-cluster/` (Ingress exclusivo de GKE: `BackendConfig`, certificado gestionado de Google, sin equivalente en AKS).
- **Gobernanza de proyecto open source** (es un fork académico, no un proyecto que reciba contribuciones externas): `.github/auto-approve.yml`, `CODE_OF_CONDUCT.md`, `CODEOWNERS`, `CONTRIBUTING.md`, `header-checker-lint.yml`, `ISSUE_TEMPLATE/`, `pull_request_template.md`, `SECURITY.md`, `snippet-bot.yml`.
- **`src/shoppingassistantservice/` + `kustomize/components/shopping-assistant/`** — 12º microservicio opcional (asistente de compra con Vertex AI/Gemini), fuera del alcance declarado de "11 microservicios", ya excluido por defecto en `kubernetes-manifests/kustomization.yaml`/`kustomize/base`.

### Mantenido a propósito aunque no se vaya a usar directamente

- **`terraform/`** (GCP/GKE actual) — se reemplaza por Terraform de Azure, pero se deja como referencia estructural (`variables.tf`/`outputs.tf`/`providers.tf`) hasta que se escriba el nuevo.
- **`helm-chart/`** — chart de Helm para desplegar la propia app Online Boutique (no confundir con el CLI `helm`, ver Tarea 14). No se usa porque el despliegue es vía Kustomize + Argo CD, pero es agnóstico de nube y no cuesta nada mantenerlo.
- **`kustomize/components/service-mesh-istio/`** y demás componentes opcionales de Kustomize (`alloydb`, `spanner`, `memorystore`, `google-cloud-operations`, `cymbal-branding`, `single-shared-session`, `custom-base-url`, `non-public-frontend`, `container-images-registry/tag/tag-suffix`, `without-loadgenerator`) — están comentados/opt-in por defecto, no interfieren con nada, y sirven de ejemplo de cómo añadir componentes a los overlays propios.
- **`src/loadgenerator/`** (Locust) y `kubernetes-manifests/loadgenerator.yaml` — se sustituye por k6, pero el `locustfile.py` sirve de referencia de los "user journeys" (navegar/carrito/comprar) al escribir el script de k6, incluido el perfil de "usuario testigo". Borrar solo cuando el script de k6 ya esté listo.
- **`.github/workflows/{kubevious-manifests-ci,helm-chart-ci,kustomize-build-ci,terraform-validate-ci}.yaml`** — no dependen de credenciales GCP. `kustomize-build-ci` y `terraform-validate-ci` de hecho coinciden exactamente con checks ya planeados en la Tarea 7 (validan por ruta, no por contenido, así que siguen funcionando cuando se sustituya el contenido de `terraform/`).
- **`.github/renovate.json5`** — sigue actualizando dependencias de los 11 microservicios (evidenciado por commits recientes de Renovate en el historial), no está atado a GCP.

---

## Tarea 3 — Documentar el alcance del TFM ✅

Cubierto por la sección "Resumen del proyecto" (arriba) y las tareas 7-15 de este mismo documento, que desarrollan cada punto del enunciado original (CI/CD, Terraform, Kubernetes/AKS, Prometheus/Grafana).

---

## Tarea 4 — Validación local de microservicios ⏳

**Por qué:** lo único de todo el plan que cuesta dinero real es el primer `terraform apply` contra Azure (AKS, ACR, VNet). GitHub Actions con runners hospedados, `docker build`, tests, linters y `kustomize build` no consumen crédito. Por eso se valida todo lo posible en local antes de aprovisionar nada. Esta tarea es la parte "por microservicio" de esa validación; el clúster local (`kind`) y Terraform son tareas propias e independientes (Tareas 5 y 6), porque necesitan que los 11 microservicios ya estén revisados, no al revés.

**Aplicación (repetir por cada uno de los 11 microservicios — es el mismo pase completo que ya se hizo con `productcatalogservice`, ver más abajo):**
- Instala dependencias y compila sin error.
- Escribir los tests que falten (ver plan detallado más abajo) y comprobar que todos — existentes y nuevos — están en verde en local antes de abrir ningún PR.
- Linter corrido en local (el mismo que luego correrá en el pipeline de CI, Tarea 7), sin sorpresas al activarlo.
- El `Dockerfile` construye limpio; verificar que la imagen final no arrastra herramientas del stage de build (compiladores, `npm`, `pip`) — `currencyservice`/`paymentservice` ya lo hacen bien de fábrica (runtime final Alpine + runtime de Node, sin npm), el resto se revisa según le toque turno. Llevarlo a nivel de producción cuando aplique (imagen base fijada por digest exacto, usuario no-root) — patrón completo ya aplicado y documentado en `productcatalogservice`.
- Si el desarrollador fuera a añadir funcionalidades a ese servicio, preparar un `Dockerfile.dev` con recarga en caliente (ej. `air` para Go) para no reconstruir la imagen en cada cambio — no imprescindible en todos, solo donde se prevea iterar activamente; ya hecho como ejemplo en `productcatalogservice`.
- (Opcional, recomendable) Escribir un `docker-compose.yaml` para levantar los 11 servicios juntos en local y navegar la tienda de extremo a extremo sin clúster — no existe, lo cubría `skaffold.yaml` (ya borrado). Se escribe al final, cuando los 11 ya estén revisados individualmente.

**Tests — estado de partida y plan por microservicio (investigado, no inventado):** de los 11, solo 5 tienen tests ya escritos — `cartservice` (C#, `tests/CartServiceTests.cs`), `checkoutservice` (Go, `money/money_test.go`), `frontend` (Go, `money/money_test.go` + `validator/validator_test.go`), `productcatalogservice` y `shippingservice` (Go). El job `code-tests` conservado de `.github/workflows/ci-pr.yaml` ya ejecuta justo esos — punto de partida sin bootstrap. Ninguno de los 6 restantes tiene framework de test declarado todavía (`loadgenerator` se descarta, va a desaparecer al migrar a k6):

- **adservice (Java/Gradle)**: añadir JUnit 5 + `test { useJUnitPlatform() }` + JaCoCo. Testear `getAdsByCategory`, `getRandomAds` y sobre todo `AdServiceImpl.getAds` (regla real: ads por categoría si hay match de contexto, si no aleatorios) — instanciable en memoria con un `StreamObserver` falso, sin red.
- **currencyservice (Node.js)**: añadir Jest. **Refactor previo necesario**: `server.js` no exporta nada y llama a `main()` sin guardas — extraer `_carry`/`convert` a un módulo aparte (o añadir `module.exports` + `if (require.main === module)`). Testear `_carry` (carry/overflow) y `convert` (con el JSON de tipos de cambio ya existente).
- **paymentservice (Node.js)**: añadir Jest. **Sin refactor** — `charge.js` ya exporta `charge()` puro. Testear tarjeta inválida, tipo no aceptado (solo visa/mastercard), caducada, éxito.
- **emailservice (Python)**: añadir pytest + pytest-cov (dependencia de test separada, no en `requirements.in` de producción). Único punto testeable real: `template.render(order=...)` de Jinja2 con un `order` de mentira — la clase `EmailService` real nunca se instancia en producción (siempre modo `DummyEmailService`), no hay más lógica de negocio que testear ahí.
- **recommendationservice (Python)**: añadir pytest. Testear `ListRecommendations`: filtrado (`set(product_ids) - set(request.product_ids)`) y el tope `min(max_responses, num_products)`, con un `product_catalog_stub` mockeado a mano.

**`productcatalogservice` — ya revisado a fondo, sirve de plantilla de referencia para el resto de los 10** (este ya tenía tests propios, así que su pase no incluyó "escribirlos", solo confirmarlos en verde — el resto del checklist sí aplica entero):
- Código y Dockerfile limpios de Google Cloud, tests en verde, linter corriendo, Dockerfile llevado a nivel de producción (digest fijado + `nonroot`), `Dockerfile.dev` con `air` para recarga en caliente. Detalle completo de cada paso en el historial de conversación; lo que queda pendiente de ese repaso se anota abajo.

**Nota (hallazgo durante la validación de `productcatalogservice`):** el servicio traía código GCP-específico sin usar: Stackdriver Profiler (activado por defecto vía `cloud.google.com/go/profiler`, sin equivalente necesario — Prometheus/Grafana, Tarea 15, no hace profiling de función) y una ruta alternativa de carga de catálogo vía AlloyDB + Secret Manager (`ALLOYDB_CLUSTER_NAME`, nunca seteada en ningún manifiesto — el componente Kustomize `alloydb` ya estaba marcado como no usado en la Tarea 2). Se eliminó el código de ambas (funciones `initProfiling`, `getSecretPayload`, `loadCatalogFromAlloyDB` y las dependencias `cloud.google.com/go/profiler`, `cloud.google.com/go/alloydbconn`, `cloud.google.com/go/secretmanager`, `github.com/jackc/pgx/v5` de `go.mod`) en vez de migrarse a Azure: ninguna aporta a la infraestructura declarada (Tarea 8) ni al experimento (k6 + fault injection). Pendiente: revisar el mismo patrón (Profiler sobre todo) en el resto de microservicios durante esta tarea. La limpieza de la propia variable `DISABLE_PROFILER`, ya inútil, en `kubernetes-manifests/productcatalogservice.yaml` se deja para la Tarea 9 (nota ahí abajo), que es cuando tocan esos manifiestos de verdad — no antes.

**Nota (`productcatalogservice`, Dockerfile ya hardened en esta tarea):** imagen final fijada por digest (`gcr.io/distroless/static:nonroot@sha256:...`) y usuario `nonroot`. Pendiente: pasar Trivy sobre la imagen ya construida (`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy image productcatalogservice:local`) — no hace falta ejecutarlo suelto ahora, ya está cubierto de forma genérica para las 11 imágenes en la Tarea 7 (Track B).

**Decisión de ejecución (validada en local durante esta tarea con `productcatalogservice`): Docker siempre que se pueda, sin sacrificar las anotaciones nativas de PR.** En vez de depender de una action de `setup-<lenguaje>` distinta por tecnología (`setup-go`, `setup-node`, `setup-python`, `setup-java`, `setup-dotnet`), instalar/compilar/testear/lintar se ejecuta dentro de contenedores oficiales (`golang:1.27.0-alpine`, `golangci/golangci-lint`, etc.) — exactamente los mismos comandos que en local, cero diferencia de entorno entre el portátil y el runner, y menos actions de terceros de las que depender (relevante para un TFM de seguridad: menos superficie de cadena de suministro). Esto **no sustituye** a las actions de reporting nativas ya decididas en la Tarea 7 (`golangci-lint-action`, `dorny/test-reporter`, etc.): esas siguen ahí porque su valor real es convertir el resultado en comentarios inline sobre la línea exacta del PR vía la API de GitHub Checks, algo que un `docker run` suelto no da (solo texto plano en el log). El patrón concreto: ejecutar con Docker → generar el fichero de salida en el formato estándar que cada action de reporting espera (JSON de `go test -json`, JUnit XML, LCOV, etc.) → pasárselo a esa action. Para `golangci-lint` ni hace falta ese paso intermedio: la action oficial ya ejecuta la herramienta directamente sin pasar por `setup-go`, así que se usa tal cual.

---

## Tarea 5 — Validación local de Kubernetes (`kind`) ⏳

**Por qué va después de la Tarea 4 y no junto a ella:** desplegar la tienda completa necesita los 11 microservicios ya construidos, no tiene sentido probar el clúster con solo alguno revisado.

- Clúster local con `kind`. **Importante**: el CNI por defecto de `kind` (kindnet) *no* aplica NetworkPolicies — instalar Calico para que el overlay hardened se comporte igual que en AKS con Azure CNI.
- Desplegar Escenario A (`kustomize/base`): los 11 pods sanos, probes en verde.
- Desplegar Escenario B (overlay hardened): comprobar que las NetworkPolicies bloquean tráfico no autorizado de verdad (ej. `curl` a la base de datos desde un pod sin permiso debe fallar) y que el tráfico legítimo sigue funcionando.
- Comprobar que un Pod que pide más recursos de los permitidos por `LimitRange`/`ResourceQuota` es rechazado.
- Ejecución de humo del script de inyección de fallos (`kubectl delete pod ...`) y de k6 (pocos VUs, pocos segundos) contra el clúster local, antes de apuntarlos a AKS.
- (Opcional) Argo CD también instalable en el mismo `kind`, apuntando su `ApplicationSet` al propio clúster, para validar la lógica de sincronización antes de lidiar con RBAC/conectividad remota real.

---

## Tarea 6 — Validación local de Terraform ⏳

- `terraform fmt -check` y `terraform validate` no necesitan cuenta de Azure con recursos.
- `terraform plan` necesita credenciales pero no crea nada — coste cero, se puede iterar el código completo revisando planes antes del primer `apply` real.

**Solo cuando las Tareas 4, 5 y 6 estén en verde se ejecuta el primer `terraform apply` contra Azure.**

---

## Tarea 7 — Pipeline de CI ⏳

**Por qué va aquí y no después de escribir Terraform/Kustomize (Tareas 8-10):** el CI no depende de que exista infraestructura en Azure, ni siquiera de que el Terraform/Kustomize definitivos ya estén escritos — se construye una vez y va creciendo de alcance solo según las Tareas 8-10 añaden contenido real:

- El **Track A** (deps, compilar, linter, tests, coverage por microservicio) solo depende de la Tarea 4 (que ya incluye escribir los tests que faltaban) — cero relación con Azure o con Terraform/Kustomize.
- `docker build` + Trivy del **Track B** ya tienen algo que validar desde ya: los 11 `Dockerfile` existen desde el fork original.
- `kustomize build` + `kube-linter` inicialmente solo validan `kustomize/base` (ya existe); en cuanto la Tarea 9 añada el overlay hardened, el mismo job empieza a validarlo también sin cambios.
- `terraform fmt`/`validate`/`plan` + tfsec/checkov/tflint inicialmente corren sobre el `terraform/` de GCP que aún no se ha sustituido; en cuanto la Tarea 8 lo reemplace por el de Azure, el mismo job (que valida por ruta, no por contenido) sigue funcionando sin tocarlo.
- La única pieza que toca Azure de verdad es `terraform plan`, y solo necesita **credenciales** (service principal/OIDC), no recursos desplegados — es de solo lectura, coste cero. Configurar esas credenciales es un trámite de una vez, no "gastar créditos".
- Lo único que de verdad gasta crédito de Azure son el pipeline de CD (Tarea 11, con su AKS de staging efímero) y el pipeline de Infra (Tarea 12, el `apply` real) — ambos ya están separados del CI.

Dos tracks en paralelo, disparados en cada Pull Request.

**Decisión de ejecución: Docker siempre que se pueda, sin sacrificar las anotaciones nativas de PR** — decidido y ya validado en la práctica durante la Tarea 4, ver esa tarea para el detalle completo del razonamiento y el patrón concreto (ejecutar con Docker → generar el formato de salida que cada action de reporting espera → pasárselo a esa action).

### A) Pipeline de aplicación (matriz por microservicio), orden fijo y su razón

1. **Checkout**
2. **Instalar dependencias** — `go mod download` / `npm ci` / `pip install -r requirements.txt` / `./gradlew dependencies` / `dotnet restore`.
3. **Compilar** — `go build ./...` / `./gradlew compileJava` / `dotnet build`. Node.js/Python no compilan (interpretados). *Va antes que el linter* porque un fallo de compilación es un corte duro más barato de diagnosticar que gastar tiempo analizando estilo de código que ni siquiera construye.
4. **Linter** (informativo, no bloqueante): Go → `golangci-lint`; Node.js (currencyservice, paymentservice) → ESLint; Python (emailservice, recommendationservice) → `ruff`; C# (cartservice) → `dotnet format --verify-no-changes`; Java (adservice) → Checkstyle (candidata a más fricción, ver más abajo).
5. **Resultados del linter** — anotaciones inline en la PR: `golangci/golangci-lint-action` (nativo), `reviewdog/action-eslint`, `astral-sh/ruff-action` (nativo), salida directa de `dotnet format`, y para Checkstyle una acción de reviewdog si la integración nativa da problemas.
6. **Tests unitarios** (detalle completo en Tarea 4). La cobertura del paso 8 se recoge **aquí mismo**, en la misma invocación (`go test -coverprofile`, `pytest --cov`, `jest --coverage`, `dotnet test --collect:coverage` generan tests y cobertura en un único comando) — el paso 8 solo reporta ese dato, no vuelve a ejecutar nada.
7. **Resultados de tests** — unificar con [dorny/test-reporter](https://github.com/dorny/test-reporter): `golang-json` (Go, sin conversión), `dotnet-trx` (.NET), `java-junit` (JUnit XML — también sirve para el XML de `pytest --junitxml`, mismo esquema). Jest genera su JUnit vía `jest-junit`.
8. **Coverage de tests** — reporting del dato ya recogido en el paso 6. Sin acción única para las 5 tecnologías, se prueba una por lenguaje: Go nativo (`go tool cover -func`), .NET (`coverlet` + `danielpalme/ReportGenerator-GitHub-Action`), Java (JaCoCo + `madrapps/jacoco-report`), Node/Jest (`ArtiomTr/jest-coverage-report-action`), Python (`pytest-cov` + `py-cov-action/python-coverage-comment-action`). Aviso: Gradle+JaCoCo+Actions es la combinación con más piezas móviles — si falla, se documenta como limitación en la memoria en vez de forzarla.

### B) Pipeline de infraestructura y manifiestos (no ligado a un servicio, en paralelo a A)

- `docker build` de las 11 imágenes sin push (valida Dockerfile) + Trivy sobre esas imágenes (informe, no bloquea aquí).
- `terraform fmt` / `validate` / `plan` (comentado en la PR) + tfsec + checkov (informe, no bloquea aquí) + `tflint` (buenas prácticas del proveedor, complementa a los dos anteriores que son de seguridad).
- `kustomize build` de los overlays A y B (valida que renderizan) + `kube-linter` sobre esa salida — detecta justo lo que mide el TFM (falta de resource limits, falta de probes, contenedores como root), generando de paso un dato comparativo extra entre A y B.
- `actionlint` sobre los propios workflows de `.github/workflows/`.

---

## Tarea 8 — Infraestructura Azure (Terraform) ⏳

- **Red privada:** VNet segmentada en subredes independientes (Ingress pública, nodos de AKS privada).
- **Clústeres AKS:** uno para Escenario A, uno para Escenario B, con plugin de red Azure CNI (para que las NetworkPolicies se apliquen) y Cluster Autoscaler con mínimo/máximo de nodos.
- **Azure Container Registry (ACR)** — hay que provisionarlo explícitamente (no estaba dicho al principio), con autenticación desde GitHub Actions vía OIDC (`azure/login`), sin credenciales estáticas.
- **Estado remoto:** backend en Azure Blob Storage con state locking.

---

## Tarea 9 — Dos escenarios de despliegue (Kustomize) ⏳

- **Escenario A** = `kustomize/base` tal cual (sin límites ni políticas de red).
- **Escenario B** = overlay propio nuevo que activa `kustomize/components/network-policies` (ya existe) + añade `ResourceQuotas` y `LimitRanges` (no existen todavía, hay que crearlos).

**Pendientes de manifiestos detectados durante la Tarea 4 (revisar al tocar cada manifiesto, no antes):**
- `kubernetes-manifests/productcatalogservice.yaml` setea `DISABLE_PROFILER: "1"`, variable que ya no lee ningún código (el Profiler se eliminó del fuente en la Tarea 4) — limpiar.
- Añadir `securityContext: runAsNonRoot: true` (y valorar `readOnlyRootFilesystem: true`) en el pod de `productcatalogservice`, para que el manifiesto **exija** lo que su Dockerfile ya soporta desde la Tarea 4 (imagen `nonroot` desde `gcr.io/distroless/static:nonroot`) — encaja de forma natural en el Escenario B (blindado), y es revisable en el resto de microservicios según cada Dockerfile vaya quedando en el mismo estado.

---

## Tarea 10 — Probes y autoescalado ⏳

Revisar/completar `livenessProbe`/`readinessProbe` en los manifiestos existentes y añadir Horizontal Pod Autoscaler (HPA) en el frontend.

---

## Tarea 11 — Pipeline de CD ⏳

**Secuencia al mergear a main:**

build + push de las 11 imágenes a ACR con tag = SHA del commit (artefacto inmutable, nunca se reconstruye después) → `terraform apply` de un AKS de **staging efímero** → deploy directo (kubectl/kustomize) + smoke test (health checks + k6 corto) → `terraform destroy` del staging (pase o no) → **gate manual de aprobación** (GitHub Environment con required reviewer) → al aprobar, se actualiza el tag de imagen en el `kustomization.yaml` de A o B (commit a git) → **Argo CD sincroniza automáticamente** (GitOps, pull-based; Terraform sigue push-based).

**Piezas añadidas tras revisar "despliegue/docker" (no quitan nada de lo anterior):**
- **ACR** provisionado en la Tarea 8 (Infra), no en el propio CD.
- **Firma de imagen + SBOM**: firmar con `cosign` y generar un SBOM (Syft o atestación nativa de `docker buildx`) — barato y muy on-tema para un TFM de seguridad. Opcional pero recomendable.
- **Ingress público**: falta un Ingress Controller real (nginx-ingress o AGIC de Azure) + `cert-manager` si se quiere TLS, para exponer el frontend. Se despliega igual que el resto de la app, vía Argo.
- **Rollback**: con GitOps es solo un `git revert` del commit que cambió el tag de imagen — Argo CD resincroniza automáticamente al estado anterior, sin mecanismo adicional.
- **Salud del despliegue**: Argo CD puede esperar a que los pods estén `Ready` (usando las mismas probes de la Tarea 10) antes de marcar la sincronización como `Healthy`.

---

## Tarea 12 — Pipeline de Infra (trigger: cambios en `terraform/`) ⏳

Aplica VNet, AKS de A y B, ACR, backend remoto (Tarea 8). tfsec/checkov es **bloqueante únicamente en el plan/apply del Escenario B** (hardened); en A solo informa, generando el dato para la comparativa de vulnerabilidades permitidas vs bloqueadas — es literalmente el eje comparativo del TFM.

---

## Tarea 13 — Pipeline de Experimento (`workflow_dispatch`, desacoplado de los deploys) ⏳

Contenedor k6 (carga + perfil "usuario testigo") junto con el script de inyección de fallos (Bash/kubectl) que elimina el pod de un microservicio crítico a mitad de carga. Ejecutado contra A y luego contra B, repetible sin necesidad de redesplegar. Deliberadamente desacoplado del CD para poder repetir mediciones sin tocar código.

---

## Tarea 14 — GitOps con Argo CD ⏳

**Decisión confirmada: Argo CD, no Flux, no ambos.**

- **Por qué Argo y no Flux:** tiene UI web (vistoso para la defensa del TFM: tiles Synced/Healthy en directo), y `ApplicationSet` encaja perfecto con el caso de uso (misma app, overlays distintos — base para A, hardened para B — desplegada a clústeres distintos desde una única definición). Reconocimiento de mercado ligeramente mayor que Flux también cuenta para el CV. Flux es más ligero en recursos y su soporte nativo de Kustomize es más "purista", pero para una defensa académica la UI de Argo inclina la balanza.
- **Por qué no los dos:** sería complejidad añadida sin aportar nada a la pregunta de investigación real.
- **Diseño:** un hub de Argo CD permanente (fuera de A y B) con un `ApplicationSet` que despliega `kustomize/base` a A y el overlay hardened a B. El "deploy" pasa a ser solo un commit en git (Tarea 11); Argo sincroniza solo. Terraform sigue siendo push-based (Argo no gestiona infra de nube).
- **Nota sobre Helm (aclaración recurrente):** el CLI `helm` sí se usa — es la forma estándar de instalar Prometheus/Grafana (`kube-prometheus-stack`) y el propio Argo CD (su chart oficial), normalmente vía el provider de Helm de Terraform. Esto es distinto de la carpeta `helm-chart/` del repo (que empaqueta la app Online Boutique en sí y no se usa, ver Tarea 2).

---

## Tarea 15 — Observabilidad (Prometheus + Grafana) ⏳

- **Captura de métricas:** consumo real de recursos de los nodos, latencias de red, tasas de error HTTP/gRPC de la app y paquetes bloqueados por seguridad.
- **Dashboard de laboratorio:** MTTR (Mean Time To Recovery) — segundos exactos que tarda Kubernetes en levantar un pod nuevo tras una simulación de fallo.
- **Usuario testigo:** perfil de usuario independiente en k6 que realiza una compra simulada de forma constante y pausada mientras el resto de usuarios virtuales estresan el sistema, para medir con precisión cuántos segundos exactos experimenta un cliente real la web caída o lenta durante la inyección de fallos.

---

## Mejoras opcionales (si sobra tiempo, no comprometidas)

- **Postgres real en Azure para `productcatalogservice`.** El servicio es puramente de lectura (`ListProducts`/`GetProduct`/`SearchProducts`, sin ningún CRUD de escritura), así que conectarlo a una base de datos gestionada de Azure en vez de `products.json` sería una mejora de infraestructura sin fricción añadida (no hay que resolver consistencia de escrituras concurrentes ni nada parecido). Puramente cosmético/demostrativo, no aporta a la pregunta de investigación — no bloquea nada si no se hace.
- **Descartado a propósito, no reconsiderar:** hacer lo mismo para el carrito (`cartservice`) — el carrito se identifica por una cookie de sesión efímera, no hay sistema de cuentas/autenticación en el proyecto, así que no hay identidad de usuario persistente a la que unir un carrito guardado en base de datos. Redis (session store en memoria con expiración) es el diseño correcto para ese caso, no una limitación a resolver.

---

## Trabajo futuro / limitaciones a mencionar en la memoria final

- **Cifrado en tránsito entre microservicios (TLS/mTLS).** Todo el tráfico gRPC interno usa credenciales `insecure` (texto plano) — decisión consciente, no descuido. El eje de seguridad medido en este TFM es **aislamiento de red** (NetworkPolicies, Escenario A vs B), no cifrado en tránsito; son capas complementarias pero solo la primera está en el alcance declarado. Hacerlo bien (mTLS con rotación automática de certificados entre 11 microservicios en 5 lenguajes distintos) requeriría un *service mesh* tipo Istio — descartado explícitamente por complejidad desproporcionada para un TFM en solitario (ver Tarea 14). Mencionar en la memoria como limitación conocida y justificada, no como algo pendiente de arreglar.

---

## Decisiones descartadas explícitamente (no reconsiderar sin motivo nuevo)

- **Entornos efímeros por Pull Request** — descartado por complejidad/coste de infraestructura (namespace dinámico, DNS/ingress temporal, gestión de credenciales) desproporcionado para un TFM en solitario. El mismo efecto de "ver el cambio antes de producción" se consigue con el staging único de la Tarea 11.
- **Proceso de releases versionado formal** (semantic-release, changelog automático) — no aporta al objetivo científico. La reproducibilidad académica se cubre con `git tag` simples al congelar cada escenario (ej. `v1.0-baseline`, `v1.0-hardened`).
- **Flux en vez de (o además de) Argo CD** — ver Tarea 14.
- **Copiar solo `src/` a un repo nuevo** — ver Tarea 1.

---

## Auditoría de justificación de tecnologías

| Tecnología | Justificación | Fuerza |
|---|---|---|
| Terraform + backend remoto | Requisito explícito del TFM (IaC, VNet segmentada, AKS) | Alta |
| GitHub Actions (CI+CD+Infra+Experimento) | Requisito explícito del TFM | Alta |
| tfsec/checkov/Trivy diferenciado A vs B | Es literalmente el eje comparativo del TFM (shift-left) | Alta |
| k6 + script de inyección de fallos | Es el experimento central del TFM | Alta |
| Prometheus + Grafana + MTTR | Requisito explícito del TFM | Alta |
| Staging efímero + build-once-promote-artifact | Protege la integridad de las mediciones de A/B frente a errores de despliegue | Alta |
| Gate manual (GitHub Environment) | Coste ~10 min de configuración, simula control real de producción | Media-alta |
| **Argo CD / GitOps** | **No afecta al resultado experimental** (MTTR, latencia y NetworkPolicies dan igual si el manifiesto llega por `kubectl apply` desde CI o por sincronización de Argo). Su valor real es narrativo/CV (UI vistosa, disciplina build-once-promote). A cambio añade superficie real: registrar A y B como clústeres remotos, RBAC/conectividad, `ApplicationSet` que aprender, cómputo extra para el hub. | **Media — la única pieza "nice to have", no "necesaria para la ciencia"** |

Si el tiempo aprieta más adelante, Argo CD es la primera pieza recortable sin invalidar ningún resultado de la comparativa A/B.

---

## Verificación / salud continua

- Tras cada fase, comprobar que `kubectl apply -k kustomize/` (o el overlay que corresponda) sigue desplegando los 11 servicios sin errores antes de pasar a la siguiente.
- Verificar que ningún workflow de `.github/workflows/` restante referencia un fichero borrado (`grep -rl` de los nombres borrados sobre `.github/` y `docs/`).
