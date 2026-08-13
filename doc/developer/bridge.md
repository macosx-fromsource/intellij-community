---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Bridge UI
---

The bridge is a backend-for-frontend HTTP server embedded in the operator. It exposes CRUD over the
`GitLabCore` custom resource (`apps.gitlab.com/v2alpha1`) and serves a single-page application (SPA)
to configure GitLab instances. The bridge is disabled by default.

This page describes how to enable the bridge, create a service account for a caller, mint a token,
and reach the UI. For the internal architecture and how to work on the code, see
[internal/bridge/CLAUDE.md](../../internal/bridge/CLAUDE.md).

## Authentication model

The bridge uses caller-identity delegation, like the old Kubernetes Dashboard. The bridge does not
use the operator service account for API calls. Instead, every request to `/api` must carry a bearer
token:

> [!note]
> This applies to the bridge running in the cluster. The `kubectl bridge` plugin runs the bridge on
> your machine and authenticates with your kubeconfig instead, so it needs no bearer token. For more
> information, see [Run the bridge as a kubectl plugin](#run-the-bridge-as-a-kubectl-plugin).

```plaintext
Authorization: Bearer <token>
```

The bridge builds a per-request Kubernetes client from that token. It then forwards the call to the
Kubernetes API server, so the API server handles authentication and authorization. The token's own
RBAC decides what the caller can do. A caller with no token receives a `401` response. An action the
caller cannot perform receives a `403` response.

As a result, each caller needs their own RBAC on the `GitLabCore` custom resource
(`gitlabcores.apps.gitlab.com`). The following sections create a service account with those
permissions and mint a token for it.

## Build the bridge

The bridge is gated behind the `bridge` Go build tag, so it is **absent from public operator
images**. Only a build produced with `-tags bridge` contains the bridge server and its SPA. CI
publishes these as separate images with a `-bridge` tag suffix: `<branch-ref-slug>-bridge` on branch
and merge request pipelines, and `latest-bridge` on the default branch. Release tags never produce a
bridge image.

Build a bridge image locally with the dedicated task or Dockerfile:

```shell
CONTAINER_CLI=docker task docker-build-bridge   # tags <image>:<TAG>-bridge
# or: docker build -f Dockerfile.bridge -t <image>:<tag>-bridge .
```

For a local operator process, build with the tag:

```shell
go build -tags bridge -o bin/manager ./cmd/manager
```

## Enable the bridge

Enabling requires a bridge build (above); `ENABLE_BRIDGE` has no effect in a public image because
the bridge is not compiled in. Set `ENABLE_BRIDGE=true`, which the operator reads in
[controllers/settings/settings.go](../../controllers/settings/settings.go). The bind address
defaults to `:8090`. To change it, set `BRIDGE_BIND_ADDRESS`.

- To enable the bridge in-cluster, deploy a `-bridge` image and set the chart value
  `bridge.enabled=true`. The manager Deployment then injects `ENABLE_BRIDGE` and
  `BRIDGE_BIND_ADDRESS` and opens the container port.

  ```shell
  export HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS)
  ARGS='--set bridge.enabled=true --set image.tag=latest-bridge' task deploy_operator
  ```

- To enable the bridge locally, set the variable when you run the tagged binary:

  ```shell
  export HELM_CHARTS=$(pwd)/charts CHART_VERSION=$(head -n1 CHART_VERSIONS)
  ENABLE_BRIDGE=true go run -tags bridge ./cmd/manager
  ```

Confirm the server started:

```shell
kubectl -n gitlab-system logs deploy/gitlab-controller-manager | grep bridge
#   -> "starting bridge server","addr":":8090"
```

> [!warning]
> The chart exposes only a container port for the bridge, with no `Service` or `Ingress`. Reach it
> with `kubectl port-forward`. Do not expose it publicly while it is a proof of concept.

## Install the v2alpha1 custom resources

The `apps.gitlab.com/v2alpha1` resources that
[ADR 26](adr/0026-design-of-v2alpha1-custom-resources.md) designs are not part of the Helm chart.
The chart is what produces the release manifests and the OLM bundle, so leaving them out keeps them
out of everything a user installs.

Install them in a development cluster:

```shell
task install_v2alpha1_crds
```

The task acts on the current `kubectl` context. Check it first, because a development cluster can
hold a real GitLab instance. It reads `$NAMESPACE` and `$NAME_OVERRIDE` to find the webhook Service,
so use the same values you deployed the Operator with.

The task adds three definitions, each with `v2alpha1` as its only version:

| Definition | Kind |
|---|---|
| `gitlabcores.apps.gitlab.com` | `GitLabCore` |
| `orbits.apps.gitlab.com` | `Orbit` |
| `datainsightplatforms.apps.gitlab.com` | `DataInsightPlatform` |

`GitLabCore` is a definition of its own, not a second version of `GitLab`. A definition carries one
kind across all of its versions, so a differently named kind needs a definition of its own. The
existing `gitlabs.apps.gitlab.com` definition is untouched, keeps `v1beta1` as its only served and
stored version, and needs no conversion webhook.

Nothing converts a `GitLab` into a `GitLabCore`. Kubernetes converts only between versions of one
definition, so an instance created through `GitLab` stays there. To work with the new resource,
start from the
[GitLabCore sample](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/config/samples/gitlabcore_v2alpha1.yaml).

The bridge reads and writes `GitLabCore` only, so install the definitions before you use it. Without
them, every API call returns an error from the Kubernetes API server.

> [!note]
> Only `GitLabCore` is reconciled, and only in a build with the `bridge` tag and `ENABLE_BRIDGE=true`.
> For more information, see [the GitLabCore reconciler](gitlabcore.md). Creating an `Orbit` or a
> `DataInsightPlatform` stores the object and nothing else happens.

## Create a service account and grant access

Create a service account and bind it to a role with the verbs the caller needs on `GitLabCore`
resources. This example grants full CRUD. For a read-only caller, drop `create`, `update`, `patch`, and
`delete`.

```shell
kubectl -n gitlab-system create serviceaccount bridge-user

kubectl create clusterrole gitlab-editor \
  --verb=get,list,watch,create,update,patch,delete \
  --resource=gitlabcores.apps.gitlab.com

kubectl create clusterrolebinding bridge-user \
  --clusterrole=gitlab-editor \
  --serviceaccount=gitlab-system:bridge-user
```

To limit the caller to a single namespace, use a `Role` and `RoleBinding` instead of the
cluster-scoped variants.

## Get a token

Mint a short-lived token for the service account with the TokenRequest API (Kubernetes 1.24 and
later):

```shell
TOKEN=$(kubectl -n gitlab-system create token bridge-user --duration=1h)
```

A kubeconfig that authenticates with a client certificate or an exec or OIDC plugin cannot be
reduced to a bearer token. In that case, use `kubectl create token <service_account>` or your OIDC
ID token instead.

## Access the bridge

Forward the port, then use the token:

```shell
kubectl -n gitlab-system port-forward deploy/gitlab-controller-manager 8090:8090
```

- Use `curl` with the token:

  ```shell
  curl -H "Authorization: Bearer $TOKEN" localhost:8090/api/v1/gitlabs
  ```

- In the SPA, open <http://localhost:8090/>, paste the token into the header token field, and select
  **Save token**. The SPA attaches the token to every API request and stores it in the browser
  `localStorage`.
- For the API documentation, open <http://localhost:8090/docs>, select **Authorize**, and paste the
  token to try requests from the documentation UI.

> [!note]
> The SPA stores the token in `localStorage`, which any script on the page can read. This is
> acceptable for the current proof of concept with short-lived tokens. Do not treat it as a
> production credential store.

## Configure a GitLab instance

The chart version field is prefilled with the latest version in the `CHART_VERSIONS` file, which the
SPA reads when it is built (see `internal/bridge/web/vite.config.ts`). The operator image builds the
SPA from the same file it fetches its charts with, so the prefilled version is one the image carries,
and the form needs no request to know it. The field stays free-form for every other version.

The form in the SPA writes one `GitLabCore` resource, in three steps:

1. **Basics**: what the instance is, and which chart deploys it.
1. **Dependencies**: the data stores the instance connects to.
1. **Overrides**: the chart values, for everything the steps above do not cover.

Moving forward checks the steps you leave and stops at the first one that does not hold up. Moving
back checks nothing, so you can look at an earlier step with a half-filled one behind you. Select a
step in the header to jump to it. **Create** and **Save** are on the last step, and they check every
step again.

Each field maps to the specification:

| Field | Resource field | Description |
|---|---|---|
| **Name** | `metadata` | Name of the resource, fixed after creation. The form creates in the `gitlab-system` namespace and offers no choice of it; a resource in another namespace is still edited where it is. |
| **Hostname** | `spec.hostname` | Fully qualified domain name the instance is reached at, such as `gitlab.example.com`. The reconciler derives the chart host values from it. |
| **Edition** | `spec.edition` | `ee` for Enterprise Edition, which runs the Free feature set until a license activates more, or `ce` for Community Edition. Defaults to `ee`. |
| **License** | `spec.license.secretRef` | Name and key of the Secret that holds the license. The license key itself never reaches the bridge or the resource. Leave both empty to run without a license. The form shows this group for Enterprise Edition only, and sends no license for Community Edition. |
| **PostgreSQL** | `spec.postgresql` | Hostname of the database server, and the Secret that holds the password of the database user. For the versions and extensions GitLab requires, see [the PostgreSQL requirements](https://docs.gitlab.com/install/requirements/#postgresql). |
| **Valkey** | `spec.redis` | Hostname of the Valkey server, and the Secret that holds its password. Redis works in its place, and the resource and the chart values both still call the field `redis`. For the versions GitLab requires, see [the Redis requirements](https://docs.gitlab.com/install/requirements/#redis). |
| **Object storage** | `spec.objectStorage` | Name and key of the Secret that holds the object storage connection. Setting it turns the consolidated object storage on, which the chart needs: artifacts, LFS, uploads, and packages are enabled with no connection of their own. The registry, Pages, and backups keep their own settings in the chart values. |
| **Chart version** | `spec.chart.version` | Prefilled with the latest version the SPA was built with. Free-form, because what renders is what the Operator image carries. Required: nothing renders without a version. An upgrade is a change of this field. |
| **Chart values** | `spec.chart.values` | Free-form YAML for everything the fields above do not cover. |

All three connections are required. The chart bundles neither PostgreSQL nor Redis, and it enables
object storage for artifacts, LFS, uploads, and packages with no connection of its own, so an
instance that leaves a group empty and does not configure it in the chart values fails to render.

The chart values are merged over the values the reconciler derives from the structured fields, and
win on conflict. Use them as an escape hatch, and prefer a structured field when one exists.

A group is all or nothing: fill every field of PostgreSQL, Redis, or the license, or leave the group
empty. The form reports an incomplete group before it sends the request.

The bridge rejects a value the definition would reject, such as a hostname that is not a domain
name, with a `422` response that names the field. The constraints are part of the OpenAPI document,
so the generated client carries them too.

## Verify the RBAC delegation

To confirm the bridge uses the caller identity rather than the operator identity, use a token whose
service account lacks a verb. For example, a read-only account that attempts a create receives a
`403` response:

```shell
# A read succeeds.
curl -H "Authorization: Bearer $TOKEN" localhost:8090/api/v1/gitlabs
# A create the caller cannot perform returns HTTP 403.
```

## Run the bridge as a kubectl plugin

The `kubectl bridge` plugin runs the bridge on your own machine instead of in the cluster. The plugin
builds its Kubernetes client from your kubeconfig, so it authenticates the same way `kubectl` does.
Client certificate, exec, OIDC, and token kubeconfigs all work. You do not create a service account,
mint a token, or paste anything into the UI.

Use the plugin when you want the UI without deploying a bridge image, or when your kubeconfig cannot
be reduced to a bearer token.

> [!warning]
> The plugin performs no authentication of its own. Anyone who reaches the port acts with your
> kubeconfig permissions. It binds loopback by default. You can bind a routable address, for example
> to run the plugin in a container, but the plugin prints a warning because that exposes full cluster
> access to the network.

Loopback keeps other machines out, but not other tabs in your browser: with no token to guess, a page
on any site you have open could otherwise `fetch` `http://127.0.0.1:8090/api/...` with your
kubeconfig permissions. As `kubectl proxy --accept-hosts` does for the same problem, the plugin
answers `403` to `/api` requests that either:

- Carry a `Host` that is neither a loopback name (`localhost`, `127.0.0.1`, `[::1]`, …) nor one you
  passed to `--accept-hosts`. This blocks DNS rebinding, where a name the attacker controls resolves
  to `127.0.0.1` so that their page counts as same-origin with the plugin.
- Look like a browser fetch made for another origin, judged by `Sec-Fetch-Site`, `Origin`, and
  `Referer`.

The SPA's own requests, the docs UI, and non-browser clients such as `curl` are unaffected. A
routable address passed to `--address` is accepted as a host too; reaching the plugin under some
other name (a DNS entry, a container host name) needs `--accept-hosts`.

### Install the plugin

Build and install the binary. The task builds the SPA first, then runs `go install`, which compiles
the SPA into the binary with `go:embed`:

```shell
task install-kubectl-plugin
```

The binary goes where `go install` puts it: `$GOBIN` when set, otherwise `$(go env GOPATH)/bin`. The
task prints the resolved path.

That directory must be on your `PATH`, because `kubectl` discovers plugins by searching `PATH` for
executables named `kubectl-<name>`. Add it to your shell profile if needed:

```shell
export PATH="$(go env GOPATH)/bin:$PATH"
```

Confirm `kubectl` found the plugin:

```shell
kubectl plugin list | grep kubectl-bridge
```

To build the binary into `bin/kubectl-bridge` without installing it, use `task build-kubectl-plugin`.

### Start the plugin

Run the plugin against your current kubeconfig context:

```shell
kubectl bridge
```

The plugin prints the identity it acts as and the URL, then opens the URL in your browser. The
header shows **Local — kubeconfig identity** instead of the token field. Stop the plugin with
<kbd>Control</kbd>+<kbd>C</kbd>.

### Plugin flags

| Flag | Default | Description |
|---|---|---|
| `--port`, `-p` | `8090` | Local port to bind. Falls back to a random free port when the port is busy. |
| `--address` | `127.0.0.1` | Address to bind. IPv6 literals such as `::1` work. A non-loopback address is allowed, with a warning. |
| `--accept-hosts` | Loopback names | Comma-separated `Host` header names to accept on `/api` besides loopback ones and the bind address. Needed to reach the plugin under another name. |
| `--context` | Current context | Kubeconfig context to use. |
| `--kubeconfig` | Standard loading rules | Path to the kubeconfig file. |
| `--no-open` | Off | Print the URL instead of opening a browser. |
| `--verbose`, `-v` | Off | Log startup details and every served request at debug level. |

For example, to use a different context on another port without opening a browser:

```shell
kubectl bridge --context staging --port 9000 --no-open
```

## Frontend development

To work on the SPA with hot-module reload, run the bridge so `:8090` is reachable. Use `task run` or
a port-forward. Then start the Vite dev server:

```shell
task frontend-dev   # http://localhost:5173, proxies /api, /openapi*, and /docs to :8090
```

For more information about the frontend workflow and regenerating the OpenAPI document and typed
client, see [internal/bridge/CLAUDE.md](../../internal/bridge/CLAUDE.md).
