---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 26. Design of the v2alpha1 custom resources
---

Date: 2026-08-03

## Status

Accepted

## Context

[ADR 24](0024-operator-custom-resources.md) commits to a new set of structured custom resources for
Bridge, with one user-facing resource per chart. It records what the new API is for, not what it
looks like. It does not define how the resources are shaped, how they reach the Helm charts, or how
the Operator applies the rendered output.

This ADR is the design behind that commitment. It outlines the new GitLab, Orbit, and Siphon
custom resources, covering the resource set, the API group and version, the shape of the
specification, and how the specification is validated and converted. How the Operator
applies the rendered chart output is not part of this decision.

## Decision

### One custom resource per chart

Every chart the Operator deploys gets a custom resource of its own. Each one is user-facing, backed
by a Helm chart, and reconciled on its own. None owns another.

The `v2alpha1` API starts with three:

| Resource | Chart |
|---|---|
| `GitLabCore` | [GitLab umbrella chart](https://gitlab.com/gitlab-org/charts/gitlab) |
| `Orbit` | [`orbit-helm-charts`](https://gitlab.com/gitlab-org/orbit/orbit-helm-charts) |
| `Siphon` | [`siphon-helm-charts`](https://gitlab.com/gitlab-org/analytics-section/platform-insights/siphon-helm-charts) |

Administrators and Bridge create and edit each resource directly. `Orbit` and `Siphon` name the
`GitLabCore` they belong to through a `spec.gitlabRef` field, which lets a namespace hold more than
one GitLab instance.

The set of resources grows as the GitLab umbrella chart is broken up and new components are
introduced. Each new chart gains a resource of its own. For example:

- If OpenBao is removed from the umbrella chart, it becomes its own CRD, because it is backed by a
  custom hand-maintained Helm chart.
- If AI Gateway is removed from the umbrella chart, it becomes a Fairway custom resource.
  Because the configuration patterns across Fairway charts are equal, there is no need for
  one CRD per Fairway chart. (Note: AI Gateway is currently not Fairway generated.)

### API group and version

The resources use the existing `apps.gitlab.com` group at version `v2alpha1`. All three are
namespace-scoped, consistent with [ADR 8](0008-operator-scoped-to-namespace.md).

All three are definitions of their own, each with `v2alpha1` as its only version. `GitLabCore` is
deliberately not a second version of the existing `gitlabs.apps.gitlab.com` definition: a
definition carries one kind across all of its versions, so a differently named kind has to be a
definition of its own. The existing `GitLab` resource is left exactly as it ships.

None of the three reach an installation. The Helm chart does not carry them, and the chart is what
renders the release manifests and the OLM bundle, so leaving them out keeps them out of everything
a user installs. `config/crd/bases` holds the generated definitions, and `task install_v2alpha1_crds`
puts them in a development cluster.

Distributing nothing keeps existing installations untouched.

The `alpha` in `v2alpha1` is doing work beyond signaling maturity. Kubernetes ranks every beta
version above every alpha version, whatever the major, so `v1beta1` outranks `v2alpha1` and stays
the version clients resolve to. Naming the version `v2beta1` would have inverted that and handed
`v2beta1` to every tool that reads without pinning a version.

### No conversion from the v1beta1 GitLab resource

`GitLabCore` and `GitLab` are separate definitions, and Kubernetes converts only between versions
of one definition. No conversion links them, and none can while the kinds differ.

Existing installations therefore have no path from `GitLab` to `GitLabCore` that the API server
provides. Moving an instance means creating a `GitLabCore` and mapping the settings across, either
by hand or through a migration the Operator performs. That path is not designed yet.

`GitLabCore` is not necessarily the final name. The kind differs from `GitLab` because the two
cannot share a definition, not because the resource is a different thing. A later iteration can
retire the `v1beta1` `GitLab` resource and give the new one that name, at which point the two
become versions of one definition again and the API server can convert between them. Renaming the
kind back is itself the step that makes automated conversion possible, so the migration and the
naming are one decision rather than two.

The structured fields still have to become chart values before anything renders. Where that mapping
lives is open: it was conversion code while `GitLabCore` was a version of `GitLab`, and it becomes
part of whatever reconciles `GitLabCore`.

### A thin structured layer over chart values

`v2alpha1` keeps the chart version and the free-form chart values that `v1beta1` accepts, and adds
structured fields on top. For this iteration the structured layer covers two settings on the
`GitLab` resource: the hostname and the license. The license is a reference to a Secret, so no
license key is stored in the custom resource.

```yaml
apiVersion: apps.gitlab.com/v2alpha1
kind: GitLab
metadata:
  name: gitlab
spec:
  hostname: gitlab.example.com
  license:
    secretRef:
      name: gitlab-license
      key: license
  chart:
    version: 10.2.0
    values:
      nginx-ingress:
        enabled: false
```

`Orbit` carries no structured settings yet. It accepts the reference to its GitLab instance, a
chart version, and chart values. [ADR 27](0027-siphon-custom-resource.md) gives `Siphon`
structured fields of its own.

```yaml
apiVersion: apps.gitlab.com/v2alpha1
kind: Orbit
metadata:
  name: orbit
spec:
  gitlabRef:
    name: gitlab
  chart:
    version: 1.4.3
    values: {}
```

The structured layer stays deliberately small. Every field it gains is a field the Operator must
map to chart values, validate, and support across chart versions. Starting with two settings lets
Bridge drive a working configuration flow while the resources are still changing shape. The
free-form values keep every setting the layer does not cover reachable in the meantime. The layer
grows as settings prove worth modeling.

### Effective values

The Operator derives chart values from the structured fields, then merges `spec.chart.values` over
that result. The free-form values win on conflict. The merged result is the effective values, and
it is what the Operator renders the chart with.

Giving the free-form values the higher precedence keeps them a working escape hatch. A structured
field that maps to the same key as a value an administrator sets does not override that
administrator's choice.

### Validation

Validation happens in two places, because the two halves of the specification have different
kinds of schema.

The structured fields are validated by the definition itself, through type constraints,
`kubebuilder` validation markers, and CEL rules. Keeping this in the schema means the API server
enforces it without a webhook, and Bridge can derive its form validation from the published
OpenAPI schema.

The free-form values are validated by a validating webhook. The webhook pulls the chart named by
`spec.chart.version`, reads the chart `values.schema.json`, computes the effective values, and
validates them against that schema. Validating the effective values rather than only
`spec.chart.values` means the webhook checks what the Operator actually renders, including the
result of the merge.

This makes the chart schema part of the admission path. The webhook needs the chart available when
a request arrives, and the chart must ship a `values.schema.json`.

### Deferred decisions

This ADR does not settle:

- The shape of `status`, including conditions and the health detail Bridge renders.
- Secret handling beyond the license reference.
- How the Operator applies the rendered chart output.
- When `v2alpha1` becomes served and stored by default, and how existing installations move to it.
- Whether `GitLabCore` takes the `GitLab` name once the `v1beta1` resource retires, and whether the
  Operator converts existing instances automatically at that point.

## Consequences

- One resource per chart lets each chart be reconciled and versioned on its own, matching the fact
  that separate teams release those charts independently. Nothing in the API expresses which chart
  versions work together, so compatibility rests with documentation and with whoever sets the
  versions.
- Tying the resource set to the chart set keeps the API aligned with how GitLab is packaged, and
  makes extracting a chart from the umbrella an API change. Each extraction adds a resource, and
  administrators who configured that component through `spec.chart.values` on the `GitLab` resource
  have to move that configuration to the new resource.
- Having one class of resource keeps a single boundary to reason about. It also means the Operator
  gains no per-component reconcile isolation inside the umbrella chart. A failure while reconciling
  the umbrella chart affects the whole `GitLab` resource, and components become independently
  reconciled only once their charts are extracted.
- Bridge reads and writes one resource per chart rather than a single resource. It gains direct
  control over each component, and it takes on the job of keeping the set consistent, including
  creating an `Orbit` with a `spec.gitlabRef` that resolves. The set grows over time, so Bridge has
  to tolerate resources it does not yet know about.
- A thin structured layer keeps the API small while it changes, and every setting stays reachable
  through the free-form values. It also means `v2alpha1` does not yet deliver the well-defined,
  fully typed contract ADR 24 describes. Bridge validates most configuration against the chart
  schema rather than against the resource schema, and cannot build a form from typed fields that
  do not exist yet.
- Letting the free-form values override the structured fields keeps the escape hatch usable, at
  the cost of a specification where a structured field does not always describe the deployed
  state. Anything reading back configuration has to read the effective values, not the structured
  fields alone.
- Validating the effective values against the chart schema catches bad configuration at admission
  time instead of at reconcile time. It also couples admission to chart availability. If the
  webhook cannot obtain a chart, writes to the resource fail, so the Operator needs the charts
  cached or bundled. `orbit-helm-charts` does not ship a `values.schema.json` today, so adding one
  is a prerequisite.
- Distributing no `v2alpha1` definition means a released Operator has no effect on existing
  installations. The version is absent from discovery, so no tool that reads without pinning a
  version changes behavior. The cost is that the definitions and the Operator that reconciles them
  are released on separate paths, so a development cluster can hold a definition the running
  Operator knows nothing about.
- Naming the version `v2alpha1` keeps `v1beta1` ahead of it in the Kubernetes version ordering, so
  installing the definitions does not change the version clients resolve to. It also sets the
  expectation that the API still moves. Promoting it to `v2beta1` later flips the preferred version,
  which is a change to plan for rather than one to arrive at by accident.
- Giving `GitLabCore` a definition of its own leaves the `GitLab` definition untouched, so
  installing the `v2alpha1` set changes nothing about how an existing instance is served or stored,
  and adds no webhook to any read path. It also means the API server offers no route between the
  two, so migrating an installation is work the Operator has to do rather than something Kubernetes
  performs.
- Nothing forces a structured field to declare how it becomes chart values at the point it is
  added. Conversion code used to enforce that. Until a `GitLabCore` controller exists, a field can
  be added to the specification and reach nothing.
- Two resources now describe a GitLab instance. Documentation has to say which one an installation
  uses, and Bridge has to know which one it is talking to.
