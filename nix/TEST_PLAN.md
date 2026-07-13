# Nix flake — local test plan

End-to-end manual test plan for the flake, runnable entirely on a local machine.
Work top to bottom: stages 1–4 are pure/offline (fast, no cluster); stage 5 is
the impure cluster flow (needs a container runtime + kind); stage 6 is teardown.

## 0. Prerequisites & one-time setup

- Nix with flakes enabled (`experimental-features = nix-command flakes`).
- A container runtime for kind (Docker Desktop / colima / podman) **for stage 5 only**.
- Stages 1–4 need no cluster and no Docker.

**Flakes only see git-tracked files** — stage the new flake files once changes are made or files added:

```
git add flake.nix nix/ scripts/deploy.sh
```

---

## 1. Evaluation & checks (pure, offline)

| # | Command | Expected |
|---|---------|----------|
| 1.1 | `nix flake check` | No errors. |
| 1.2 | `nix flake show` | Lists `packages` (chart-deps, gitlab-charts, operator-manifest, operator-manifest-dev, cr-overlay, manager, image, default) and `apps` (generate, manifests, fmt, vet, lint, build, test, retrieve-charts, build-operator, deploy, deploy-dev, refresh-dev, up-dev, kind-up, kind-down, deps-dev, load-image-dev). |

---

## 2. Pure builds (offline, cacheable)

| # | Command | Expected / verify |
|---|---------|-------------------|
| 2.1 | `nix build .#operator-manifest && grep -c '^kind:' result` | Builds; multiple `kind:` docs. `grep image: result` shows the image string ending `:latest`. |
| 2.2 | `nix build .#operator-manifest-dev -o result-dev && grep -e 'image:' -e 'imagePullPolicy' result-dev` | Image string ends in `:dev` and `imagePullPolicy: "Never"`. |
| 2.3 | `nix build .#cr-overlay && cat result` | YAML overlay with `spec.chart.values.global.ingress.enabled: true`, ports 32022/32443. |
| 2.4 | `nix build .#manager && file result/bin/manager` | A binary named `manager`; on macOS it is the host arch. |
| 2.5 | `nix build .#gitlab-charts && ls result/*.tgz` | One `.tgz` per line in `CHART_VERSIONS`. |
| 2.6 | `nix build .#chart-deps && ls result/*.tgz` | Subchart tarball(s) present (e.g. cert-manager). |
| 2.7 | `nix build .#image` then run the command in the note below | `.#image` is a *streamer script*; piping it lists image layers (no Docker needed). |

> 2.7 uses pipes, which don't fit in the table above — run it directly:
>
> ```
> nix build .#image && ./result | tar tf - | head
> ```

> If 2.1/2.2 fail to render, the shared `mkOperatorSetFlags` helper is the first
> suspect. If 2.4 fails on a `vendorHash` mismatch, re-bootstrap per `nix/README.md`.

---

## 3. Dev shell & tooling parity

| # | Command | Expected |
|---|---------|----------|
| 3.1 | `nix develop -c go version` | Prints a Go version (resolved from `mise.toml`). |
| 3.2 | `nix develop -c helm version` | Prints a Helm version. |
| 3.3 | `nix develop -c kind version` | Prints a kind version. |
| 3.4 | `nix develop -c golangci-lint version` | Prints a golangci-lint version. |
| 3.5 | Parity: compare `nix develop -c sh -c 'command -v helm'` with the helm used by `nix run .#build-operator` | Both resolve to the same `/nix/store/...helm` path (dev shell == ops). |

---

## 4. Quality / codegen / test apps (offline)

These mutate the working tree or run Go — review `git status` after.

| # | Command | Expected |
|---|---------|----------|
| 4.1 | `nix run .#fmt` | `go fmt ./...`; no error. |
| 4.2 | `nix run .#vet` | `go vet ./...`; no error. |
| 4.3 | `nix run .#lint` | golangci-lint runs; writes `gl-code-quality-report.json`. |
| 4.4 | `nix run .#generate` then `git status` | controller-gen runs; deepcopy code regenerated (ideally no diff). |
| 4.5 | `nix run .#manifests` then `git status` | CRD/RBAC/webhook manifests regenerated under `config/`. |
| 4.6 | `nix run .#test` | Unit tests via ginkgo. `HELM_CHARTS` defaults to the `gitlab-charts` derivation (no `retrieve-charts` needed); `SKIP_ENVTEST=yes` by default. |
| 4.7 | `SKIP_ENVTEST=no nix run .#test` | Controller tests run with envtest (`setup-envtest` provides `KUBEBUILDER_ASSETS`). Slower. |

Escape hatches (impure, write into the repo):

| # | Command | Expected |
|---|---------|----------|
| 4.8 | `nix run .#build && file bin/manager` | Local `go build` into `./bin/manager`. |
| 4.9 | `nix run .#retrieve-charts && ls charts/*.tgz` | Charts fetched into `./charts`. |
| 4.10 | `nix run .#build-operator && head .build/operator.yaml` | Writes `.build/operator.yaml` via the shared `--set` helper. |

---

## 5. Impure cluster flow (needs container runtime + kind)

Start your container runtime first (e.g. `colima start` or Docker Desktop).

### 5a. One-shot

| # | Command | Expected |
|---|---------|----------|
| 5.1 | `nix run .#up-dev` | Runs: kind cluster (create if missing) → deps-dev → build+load image → deploy operator + GitLab CR (local dev image). Prints the `==>` step banners and the detected `KIND_LOCAL_IP`. |

### 5b. Step-by-step (equivalent to `up`, for isolating failures)

The cluster must come first — `deps-dev` installs into the cluster and uses the
current kube context, which `kind-up` points at the new cluster.

| # | Command | Expected / verify |
|---|---------|-------------------|
| 5.2 | `nix run .#kind-up` | Creates kind cluster `gitlab`. Verify: `kubectl config get-contexts \| grep kind-gitlab`. |
| 5.3 | `nix run .#deps-dev && test -f external-deps.yaml` | Provisions external deps into the cluster; writes `external-deps.yaml`. |
| 5.4 | `nix run .#load-image-dev` | Streams the operator image into kind. Verify: `docker exec gitlab-control-plane crictl images \| grep gitlab-operator`. |
| 5.5 | `nix run .#deploy-dev` | Sequences cert-manager + operator install, then applies the merged GitLab CR (local dev image). |

### 5c. Assertions

| # | Command | Expected |
|---|---------|----------|
| 5.6 | `kubectl --context kind-gitlab -n gitlab-system get deploy gitlab-controller-manager` | Deployment present and becomes Available. |
| 5.7 | `kubectl --context kind-gitlab -n gitlab-system get pods` | Operator pod Running; using the `:dev` image (`kubectl ... get pod <op> -o jsonpath='{..image}'`). |
| 5.8 | `kubectl --context kind-gitlab -n gitlab-system get gitlab -o yaml \| yq '.items[0].status'` | The GitLab CR is accepted and its `status.phase` progresses (reconciliation started). |

> Full GitLab bring-up is heavy and may not complete on a laptop — the
> meaningful assertions here are: operator Running on the local image + CR
> accepted + reconciliation progressing. Full GitLab + smoke testing is the
> deferred VM/e2e work.

### 5d. Inner-loop refresh

| # | Command | Expected |
|---|---------|----------|
| 5.9 | edit operator code, then `nix run .#refresh-dev` | Rebuilds + reloads the image and `rollout restart`s the operator; `rollout status` returns success. New pod runs the rebuilt image. |

### 5e. Negative checks

| # | Command | Expected |
|---|---------|----------|
| 5.10 | `mv external-deps.yaml /tmp/ && nix run .#deploy-dev; mv /tmp/external-deps.yaml .` | Fails fast with "external-deps.yaml not found — run 'nix run .#deps-dev' first." |

---

## 6. Teardown

| # | Command | Expected |
|---|---------|----------|
| 6.1 | `nix run .#kind-down` | Deletes the kind cluster `gitlab`. |
| 6.2 | `rm -rf result result-dev bin charts .build external-deps.yaml` | Clean working tree (these are all gitignored build artifacts). |

---

## Pass criteria

- Stages 1–4 succeed offline with no cluster.
- Stage 3.5 shows dev shell and apps resolving the **same** tool store path.
- Stage 5 brings the operator up on the locally-built image and the GitLab CR is
  accepted and reconciling; `dev-refresh` rolls a code change without manual steps.
