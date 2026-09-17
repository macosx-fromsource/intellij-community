---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: The GitLabCore reconciler
---

`internal/controller/gitlabcore` reconciles the `apps.gitlab.com/v2alpha1` `GitLabCore` resource,
the resource Bridge configures a GitLab instance through. For the design of the resource, see
[ADR 26](adr/0026-design-of-v2alpha1-custom-resources.md).

The reconciler is alpha. The `controllers` and `helm` packages keep serving the `v1beta1` `GitLab`
resource, and the two paths share no code.

## Enable the reconciler

The reconciler rides with Bridge and is gated twice over, like the Bridge server itself:

- The `bridge` build tag compiles it in. Without the tag, `gitlabcore_stub.go` takes its place and
  neither the reconciler nor the `v2alpha1` types reach the binary, so public images carry neither.
- `ENABLE_BRIDGE=true` registers it at runtime, even in a tagged build. This gate is not only a
  feature switch: the `GitLabCore` definition ships in no release, and a watch on a definition the
  cluster does not serve fails the manager on start.

`task install_v2alpha1_crds` installs the definitions and grants the manager ServiceAccount what
the reconciler needs beyond the chart permissions: `gitlabcores`, its `status` and `finalizers`
subresources, and `PodDisruptionBudgets`. The chart grants the `v1beta1` GitLab permissions only, and
no `PodDisruptionBudget` permissions at all, because the `v1beta1` controller applies none. The grant
goes into a role of its own, so `task deploy_operator` does not revert it.

Install both and run the Operator against a development cluster:

```shell
task install_v2alpha1_crds
ENABLE_BRIDGE=true HELM_CHARTS=$(pwd)/charts task run_bridge
kubectl apply -f config/samples/gitlabcore_v2alpha1.yaml
```

`task run` builds without the tag, so the reconciler is absent there. Build an image with
`task docker-build-bridge`, which uses `Dockerfile.bridge`.

The sample points at no infrastructure. For a resource that is wired to PostgreSQL, Redis, and object
storage, run `bash scripts/dev_dependencies.sh setup`: it provisions them and writes an
`external-deps-v2alpha1.yaml` to apply as it is. For more information, see
[External dependencies](installation.md#external-dependencies).

### Reconciler settings

These `controllers/settings` environment variables configure the dynamic chart pull (see
[What one reconcile does](#what-one-reconcile-does)). None of them apply to the `v1beta1`
controller, which never reaches out to a chart repository.

| Variable | Default | Meaning |
|---|---|---|
| `ENABLE_DYNAMIC_CHART_PULL` | `true` | Falls back to a pull when `HELM_CHARTS` does not carry `spec.chart.version`. Set to `false` for a cluster that must not reach out to the network. The version is then a configuration error instead. |
| `DYNAMIC_CHART_REPOSITORY` | `https://charts.gitlab.io/` | The Helm chart repository (an `index.yaml` repository, not an OCI registry) a pull downloads from. Must be an `https://` URL unless `DYNAMIC_CHART_ALLOW_HTTP` overrides that. |
| `DYNAMIC_CHART_ALLOW_HTTP` | `false` | Allows `DYNAMIC_CHART_REPOSITORY` to be a plain `http://` URL. Leave this off unless the repository is a disconnected cluster's own internal mirror that only serves plain HTTP. |
| `DYNAMIC_CHART_CACHE_DIRECTORY` | `<os.TempDir()>/gitlab-operator-charts` | Where a pulled chart is cached on disk, so a repeated reconcile does not download it again. The manager container must be able to create and write to this directory. A `containerSecurityContext.readOnlyRootFilesystem: true` manager needs a writable volume mounted there; the chart mounts an `emptyDir` at the default path when `bridge.enabled`. A custom directory needs a volume of its own. |
| `DYNAMIC_CHART_CACHE_TTL` | `30m` | How long a chart may sit in the cache directory, unused, before it is pruned. A Go duration, for example `72h`. `0` disables pruning. A value with no unit, such as `30`, fails to parse. `Load` logs that on stderr and keeps the previous value rather than silently taking it. |

## What one reconcile does

1. Reads the capabilities of the cluster with `internal/render/capabilities`.
1. Renders the GitLab chart with `internal/render`, using the release name and namespace of the
   resource.
1. Runs the `pre-install` hooks with `internal/render/hookexec`, RBAC excluded, and only when the
   hooks of this release have not run yet.
1. Applies the rendered objects server-side, definitions and RBAC excluded.
1. Reports the readiness of the rendered `Deployments` and `StatefulSets` through
   `status.conditions`.

`pre-install` is the only hook event that runs. The post-install hooks of the chart belong to the
NGINX admission webhook patch and the Traefik dashboard, and the Operator overrides both subcharts
off, so no release renders one. A test asserts this against every chart version the Operator
carries.

The hooks run once per release, not once per pass. `ConditionInitialized` carries the generation
whose hooks completed, and the generation covers the whole specification, so the hooks run again
when an administrator changes the resource and not otherwise. An apply that fails is retried
without them.

This matters because of what a rerun costs. The shared secrets Job carries
`helm.sh/hook-delete-policy: hook-succeeded,before-hook-creation`, so running the event again
deletes the Job, recreates it, and waits for a pod to complete.

Every successful reconcile asks for the next one, 30 seconds later, so the loop runs for as long as
the resource exists. Nothing else triggers it: the reconciler watches the `GitLabCore` resource and
none of the objects it applies. A release is hundreds of objects, of kinds the Operator does not
know ahead of time, and a reconcile renders the whole chart, so a watch on them would mean an
informer per kind and a full render on every status update they make. An object that is deleted or
edited by hand is restored on the next pass instead, within the requeue delay.

The chart comes from the charts directory of the Operator, which `HELM_CHARTS` points at and the
image bakes in. When that directory does not carry `spec.chart.version`, the reconciler falls back
to pulling that version from a chart repository (`internal/render.PullChart`), rather than failing
outright. `ENABLE_DYNAMIC_CHART_PULL` turns this off, for a cluster that must not reach out to the
network; the version is then a configuration error, and the error names the versions the Operator
does carry. See [Reconciler settings](#reconciler-settings) for the repository, cache, and protocol
settings, and [Known limits](#known-limits) for what a pulled chart is not checked against.

The cluster is the only source of capabilities. `GITLAB_OPERATOR_KUBERNETES_VERSION` and
`GITLAB_OPERATOR_KUBERNETES_API_VERSIONS` configure the frozen renderer of the `v1beta1` path and do
not reach the reconciler. A render that claims capabilities the cluster does not have produces objects
that cannot be applied, and applying them is what this reconciler does with the result.

Every object is applied under the field manager `gitlabcore-controller`, with ownership forced. A
field another manager took is taken back, and a field the chart stops rendering is removed. For the
rules a consumer of `internal/render` follows, see
[Apply rendered objects](render.md#apply-rendered-objects).

### Effective values

`EffectiveValues` builds the values in three layers: the values derived from the structured fields,
then `spec.chart.values` merged over them, then the Operator overrides.

| Field | Chart values |
|---|---|
| `spec.hostname` | `global.hosts.gitlab.name`, and `global.hosts.domain` from the parent domain |
| `spec.edition` | `global.edition` |
| `spec.license.secretRef` | `global.gitlab.license.secret`, `global.gitlab.license.key` |
| `spec.postgresql` | `global.psql.host`, `global.psql.password.secret`, `global.psql.password.key` |
| `spec.redis` | `global.redis.host`, `global.redis.auth.secret`, `global.redis.auth.key` |
| `spec.objectStorage` | `global.appConfig.object_store.enabled`, `global.appConfig.object_store.connection.secret`, `global.appConfig.object_store.connection.key` |
| `spec.openbao` | `global.openbao.enabled`, `openbao.install`, `global.openbao.psql.host`, `global.openbao.psql.password.secret`, `global.openbao.psql.password.key`, and optionally `global.openbao.psql.port`/`database`/`username`; `openbao.serviceAccount.name`, `openbao.serviceAccount.create: false`, `openbao.role.create: false` |

A hostname of `gitlab.example.com` therefore yields a domain of `example.com`, and with it the
sibling hosts `registry.example.com` and `kas.example.com`. An apex hostname is its own domain,
because stripping its first label would leave the public suffix.

`spec.edition` selects the image repository every component pulls from. It defaults to `ee`, which
runs the Free feature set until a license activates more. Pick `ce` only for an instance that must
carry no proprietary code.

Since chart version 10, the chart bundles neither PostgreSQL nor Redis, so `spec.postgresql` and
`spec.redis` point at servers you run. Valkey stands in for Redis. Each maps a host and the Secret
that holds the password. The port, the database, and the user keep their chart defaults, so an
instance that needs another one sets it in the free-form values. For the versions and extensions
GitLab requires, see [the PostgreSQL requirements](https://docs.gitlab.com/install/requirements/#postgresql).

`spec.objectStorage` names the Secret that holds the object storage connection, and naming it also
turns the consolidated object storage on: the connection configures nothing while it is off. The
chart enables object storage for artifacts, LFS, uploads, and packages, and gives them no connection
of their own, so a resource without this field and without the equivalent free-form values fails its
`checkConfig` with `the connection property can not be empty`. The
Secret carries the endpoint, the region, and the credentials in the
[connection format](https://docs.gitlab.com/charts/charts/globals/#connection) of the chart, and the
Operator passes it through without reading it. The registry, the Pages daemon, and the backup
toolbox read settings of their own, which stay in the free-form values.

`spec.openbao` backs the GitLab Secret Manager with an OpenBao instance, and naming it also turns on
both the GitLab-side integration (`global.openbao.enabled`) and the bundled OpenBao subchart
(`openbao.install`). OpenBao needs a PostgreSQL database of its own: it does not inherit the password
of `spec.postgresql`, so `spec.openbao.postgresql` is a connection of its own, with `passwordSecretRef`
required rather than defaulted. The port, the database, and the username fall back to the chart's own
defaults (`5432`, `openbao`, `openbao`) when left unset. See
[the OpenBao chart setup](https://docs.gitlab.com/charts/charts/openbao/#setup-gitlab-secret-manager-and-openbao)
for what those values configure; the database and its role are still an administrator prerequisite,
the same way `spec.postgresql`'s database is.

`spec.openbao.serviceAccount.name` names the ServiceAccount OpenBao's pod runs as, and defaults the
chart's own `openbao.serviceAccount.create` and `openbao.role.create` to `false`. Both default `true`
in the chart and would otherwise create a Role granting `get`/`update`/`patch` on Pods and a
RoleBinding to it; the Operator does not manage RBAC on the cluster it reconciles (mirroring the
shared secrets ServiceAccount below), so that Role and its RoleBinding are an administrator
prerequisite instead, created ahead of time for the named ServiceAccount.

The derived layer also mirrors the shared secrets defaults of the `v1beta1` controller:

| Default | Reason |
|---|---|
| `shared-secrets.serviceAccount.create: false`, `name: $GITLAB_MANAGER_SERVICE_ACCOUNT` | The Job runs under the ServiceAccount of the Operator, which the Operator installation provisions. |
| `shared-secrets.rbac.create: false` | The Operator creates no RBAC, and the Job needs none: its account already has it. |
| `shared-secrets.securityContext.runAsUser: ""`, `fsGroup: ""` | Keeps the Job compatible with the OpenShift `nonroot` SecurityContextConstraint, which assigns both itself. |
| `registry.enabled: false` | The container registry keeps its images in object storage of its own, through `registry.storage`, which no structured field covers. An instance that wants one turns it back on in `spec.chart.values`, where it also supplies the storage. |

That ServiceAccount has to exist in the namespace of the resource, with permission to manage Secrets
there, before the first reconcile. The `v1beta1` controller reaches the same result by applying only
the ConfigMap and the Job of that component.

The free-form values win over the derived ones, as ADR 26 decides, which keeps them a working escape
hatch. The Operator overrides win over both:

| Override | Reason |
|---|---|
| `installCertmanager: false` | cert-manager is a prerequisite of the Operator. The cluster administrator installs it once, and it serves every instance. |
| `gitlab-runner.install: false` | The GitLab Runner has a lifecycle of its own and is deployed through the Runner Operator. |
| `nginx-ingress.enabled: false`, `nginx-ingress-geo.enabled: false`, `haproxy.install: false`, `traefik.install: false`, `global.gatewayApi.installEnvoy: false` | The Operator installs no networking controllers/Operators. |

An override is a setting an instance may not choose, because the Operator, not the chart, owns what
it configures. Keep the list short: every entry is a value an administrator sets and does not get.

### Status

| Condition | Meaning |
|---|---|
| `Initialized` | The chart resolved and rendered, and its hooks ran. |
| `Available` | Every rendered workload has its desired replicas ready. |

`status.phase` reports `Preparing`, `Running`, or `Failed`, `status.version` records the chart
version that was applied, and `status.gitlabVersion` the version of GitLab that chart deploys, such
as `19.3.2`. Both versions describe what was applied rather than what the spec asks for, so a
resource stepping through a multi-minor upgrade reports the version it has converged to so far.

The application version is read off the render, not from the catalog of charts the Operator carries:
a release can be rendered from a chart pulled at reconcile time, which the catalog knows nothing
about. Chart 10.3 and later label the migrations Job `gitlab.com/target-version`, which is
authoritative because the chart computes it and it accounts for a `global.gitlabVersion` override;
earlier charts, and releases with the migrations component disabled, fall back to the `appVersion`
the chart declares. This is the only place the application version is published, and `Siphon` reads
it to pin its table definitions.

## The reconciler installs no definitions and no RBAC

Two classes of rendered object are never applied. The cluster administrator provisions both.

- **Definitions.** No `CustomResourceDefinition` is applied, neither the ones the chart renders from
  templates, such as the Gateway API and Envoy Gateway definitions, nor the ones `Result.CRDs`
  carries from the `crds/` directories. A definition is cluster-wide and outlives every release that
  uses it, so installing one from a namespaced resource would let one GitLab instance change an API
  that another instance, and other operators, depend on.
- **RBAC.** Nothing in the `rbac.authorization.k8s.io` group is applied: no `Role`, `RoleBinding`,
  `ClusterRole`, or `ClusterRoleBinding`. RBAC grants permissions, so applying it would turn the
  right to write a `GitLabCore` into the right to grant any permission the Operator holds, which is
  cluster-wide.

An object that needs an API the cluster does not serve fails to apply, with the kind named in the
error and in the `Available` condition. Either have the cluster administrator install that API, or
turn the component off in `spec.chart.values`. The chart routes through the Gateway API by default,
so a cluster without the Gateway API and Envoy Gateway needs `global.gatewayApi.enabled: false`.

A component whose RBAC is missing starts and then fails against the API server, which no reconcile
repairs either.

The hooks go through the same filter. The chart declares its RBAC as hooks too, and the Operator has
no permission to create it, so a release whose hook RBAC was applied would fail on the first hook:

```plaintext
roles.rbac.authorization.k8s.io "gitlab-shared-secrets" is forbidden: User
"system:serviceaccount:gitlab-system:gitlab-manager" cannot delete resource "roles"
```

The shared secrets Job does not need that RBAC, because the mirrored defaults run it under the
ServiceAccount of the Operator.

## Deletion

Objects in the namespace of the resource carry a controller reference to it, so Kubernetes deletes
them. A `GatewayClass` cannot be, because it is cluster-scoped: the API server rejects an owner
reference from a namespaced resource to a cluster-scoped object. A finalizer deletes it by the
release labels `internal/render` stamps:

```plaintext
operator.gitlab.com/release-name        the name of the resource
operator.gitlab.com/release-namespace   its namespace
```

`GatewayClass` is a fixed target, not one a render discovers: it is the only cluster-scoped kind
the chart ever applies through the normal object pipeline. Everything else cluster-scoped a
subchart could render is RBAC or a `CustomResourceDefinition`, and the reconciler never applies
either (see [The reconciler installs no definitions and no RBAC](#the-reconciler-installs-no-definitions-and-no-rbac)).
Deleting a resource therefore never depends on resolving or rendering its chart, dynamic chart
pull included.

The sweep is best effort and never blocks the deletion: a cluster that does not serve the Gateway
API at all has nothing to sweep, which is logged rather than treated as a failure to clean up.

## Known limits

- A reconcile blocks while the hooks of an event run. The shared secrets Job dominates the first
  reconcile of an instance, and its worker is held for that time.
- Nothing reruns the hooks when their output is gone. Deleting a generated Secret by hand leaves the
  instance without it until an administrator changes the resource, because only that changes the
  generation the marker compares against.
- The loop polls. Drift is repaired within the requeue delay rather than on the event that caused
  it, and each pass renders the chart and applies every object again, whether or not anything
  changed.
- Nothing runs the `pre-upgrade` or `pre-delete` hooks, and no upgrade path is modeled: every render
  is composed as revision 1 of an install.
- The chart `values.schema.json` is not validated against the effective values yet. That is the job
  of the validating webhook ADR 26 describes.
- A chart the dynamic pull downloads (see [Reconciler settings](#reconciler-settings)) is not
  signature- or provenance-verified: the repository is trusted out of band, the same trust the
  Operator already places in its bundled charts.

## Run the tests

The unit tests are Ginkgo specs that need the chart archive on disk and no cluster:

```shell
task retrieve-charts
HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS) \
  task unit-tests TEST_PKGS="./internal/controller/..."
```

### Run the end-to-end test

`internal/controller/gitlabcore/e2e_test.go` creates a `GitLabCore` in a throwaway namespace, calls
`Reconcile` the way the manager does, and asserts on what reaches the cluster: the secrets the hooks
generated, the owner references and release labels of the applied objects, the status conditions, and
the sweep the finalizer performs. It is gated behind the `e2e` build tag, so the unit tests never
pick it up.

Point kubectl at a throwaway cluster and install the definitions first, otherwise the test skips:

```shell
task install_v2alpha1_crds
task e2e-tests TEST_PKGS="./internal/controller/..."
```

The release renders into the test namespace and nowhere else: NGINX, Prometheus,
and the Gateway API are off through the values, and cert-manager and the Runner through the Operator
overrides, so every object is namespaced and of a built-in kind. The pods never become ready, because there is no PostgreSQL,
Redis, or object storage, which the test asserts rather than waits for.

| Variable | Description |
|---|---|
| `E2E_CLUSTER_SCOPED=1` | Enables the NGINX Ingress controller of the chart, so the release also carries RBAC and cluster-scoped objects. The `IngressClass` and the admission webhook are applied, the RBAC is skipped, and the finalizer sweeps what it applied. |
| `E2E_KEEP_NAMESPACE=1` | Keeps the namespace and the cluster-scoped objects for inspection. |

After a run with `E2E_KEEP_NAMESPACE=1`, delete the cluster-scoped objects by hand. An admission
webhook whose backing service is gone rejects unrelated writes across the whole cluster.
