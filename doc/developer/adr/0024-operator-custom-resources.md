---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 24. Structured custom resources for Operator Bridge
---

Date: 2026-07-06

## Status

Accepted

## Context

[ADR 23](0023-operator-bridge.md) establishes that Bridge and GitLab Operator are decoupled and
communicate exclusively through custom resources. To support this, the Operator needs a new API
that models the settings Bridge exposes to administrators.

The existing `apps.gitlab.com/v1beta1` custom resource accepts a free-form set of Helm chart
values ([ADR 4](0004-integration-of-the-gitlab-chart.md)) and does not provide the structured,
well-defined fields Bridge requires to drive the UI and validate input. [ADR 17](0017-structured-spec-for-gitlab-cr.md)
already committed to a structured specification for the GitLab custom resource; this ADR builds
on that direction and defines how the resources are structured to serve Bridge.

## Decision

We introduce a new set of structured custom resource definitions that accommodate the needs of
Bridge. The API is shaped by the following decisions:

- **Structured fields.** The new API exposes structured, typed fields that correspond directly
  to the settings available in Bridge. This gives Bridge a well-defined contract to read from and
  write to, and allows the Operator to validate configuration at the API level.
- **Helm charts at reconciliation time.** The Operator continues to use the GitLab Helm charts to
  reconcile the desired state, as it does today. The structured resources are translated into
  chart values internally; the charts remain the deployment mechanism.
- **One resource per chart.** Each chart the Operator deploys gets a custom resource of its own.
  All resources are peers. Each one is user-facing, is reconciled on its own, and none owns
  another. Bridge writes to every one of them. The set grows as the umbrella chart is broken up,
  because a component that moves to a chart of its own gains a resource of its own. This keeps
  reconciliation modular and lets us replace a bundled component with a
  [Fairway-based](https://gitlab.com/gitlab-com/gl-infra/platform/runway/fairway) Helm chart by
  adding a resource for the new chart.
- **Compatibility.** Existing installations based on the `apps.gitlab.com/v1beta1` custom resource
  need a path to the new API. That path is designed alongside the new resources rather than after
  them, and stays disabled while the API is still changing.

[ADR 26](0026-design-of-v2alpha1-custom-resources.md) is the design behind these decisions. This ADR
records what the new API is for and the rules it follows. ADR 26 records what the resources look
like: which ones exist and the chart behind each, the API group and version, the shape of the
specification, and how the specification is validated and converted.

## Consequences

- Bridge interacts with one resource per chart. It gains direct control over each component, and it
  takes on the job of keeping the set consistent.
- Tying the resource set to the chart set keeps the API aligned with how GitLab is packaged. Adding
  a resource is how a new chart enters the API, so the set changes as the umbrella chart is broken
  up.
- Each resource is reconciled on its own, at the cost of more resources to manage and reason about
  than a single resource covering a whole instance.
- Retaining the Helm charts as the reconciliation mechanism avoids re-implementing GitLab
  deployment logic, but ties the structured API to what the charts can express.
- The new API supersedes `apps.gitlab.com/v1beta1`. Designing the migration alongside the new
  resources keeps a path available from the start, and leaving it disabled keeps existing
  installations untouched until the API settles.
