---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 27. Siphon custom resource
---

Date: 2026-08-26

## Status

Accepted

## Context

[ADR 24](0024-operator-custom-resources.md) gives every chart the Operator deploys a custom resource
of its own. [ADR 26](0026-design-of-v2alpha1-custom-resources.md) designs that set and names
`Siphon` as one of the three, backed by the Siphon chart. It does not shape the resource.

Siphon became deployable since. Orbit ADR 001 ships Orbit and the Data Insight Platform as
independent charts, and the platform ships as the Siphon chart. A proof of concept ran the pipeline
end to end in July 2026, reaching Siphon through a `GitLabDependencies` resource that also
provisioned CloudNativePG, NATS, and ClickHouse. It exposed four problems worth designing
against. Deleting the resource
garbage-collected the data stores. The configuration was validated only at reconcile time. The
status reported ready over a pipeline that was replicating nothing. Siphon readiness and GitLab
availability depended on each other.

## Decision

`Siphon` joins `apps.gitlab.com/v2alpha1` as a namespace-scoped peer of `GitLabCore` and `Orbit`,
backed by the Siphon chart.

The resource is shaped by the following decisions:

- **Reference the dependencies rather than provision them.** The specification points at an existing
  PostgreSQL source, NATS server, and ClickHouse sink. The source database objects, the NATS server,
  and the ClickHouse target tables stay administrator prerequisites. Each belongs to a system with a
  lifecycle of its own, and an Operator trusted to create them is an Operator trusted to delete
  them. The deletion path of the proof of concept was the most serious defect it carried.
- **Fix the deployment topology.** The Operator derives the single-shard layout a GitLab
  Self-Managed instance runs, and exposes no fields for it. Replica counts are not modeled, because
  the components hold locks and additional replicas buy failover rather than throughput. Chart
  values remain the escape hatch for a sharded topology, because changing the layout reassigns
  tables and is a migration rather than a tuning change.
- **Model the table definitions in the resource.** The definitions are versioned with the instance,
  so selecting them belongs to the API rather than to chart values. The Operator supports both the
  image volume the chart declares and an extraction path of its own, and selects between them by
  Kubernetes version. It publishes the extracted definitions to an object it owns rather than to a
  second custom resource, as the proof of concept did.
- **Bake the chart into the Operator image.** The Operator fetches the Siphon chart alongside the
  GitLab charts, so rendering stays offline. A chart version the Operator does not carry is a
  configuration error rather than a download.
- **Report only what the Operator can observe.** Availability gates on the components that report
  readiness honestly, and excludes the one that answers unconditionally. The Operator does not claim
  that change data is flowing, because it cannot observe that.
- **Never let `GitLabCore` readiness depend on Siphon.** Siphon waits for the instance it
  references. The reverse deadlocks, which is what the proof of concept hit. As peers under ADR 24
  this holds by construction.

This ADR records what the resource is and the rules it follows. For how the reconciler behaves, see
[the Siphon reconciler](../siphon.md).

## Consequences

- Referencing rather than provisioning keeps the Operator out of three lifecycles it does not own.
  The cost is that a misconfigured prerequisite surfaces as a running pipeline that replicates
  nothing rather than as a validation error. That is why the reporting gap above matters.
- Deleting a `Siphon` leaves the replication slot, the publication, and the NATS stream behind.
  PostgreSQL retains write-ahead log for an inactive slot indefinitely, so a forgotten slot
  eventually fills the volume of the source server. The Operator records all three names and warns
  about them, but it cannot prevent the problem. A `deletionPolicy` field that could not honor
  `Delete` would be worse than no field at all.
- A fixed topology makes a second shard a chart-values change rather than an API change. If sharded
  deployments become normal, the API has to grow to describe them.
- Folding the table definitions in puts the first outbound network call on a reconcile path. An
  installation needs egress to the registry, and an air-gapped one has to mirror the image or select
  the image volume.
- The extracted definitions have to fit a single ConfigMap, which cannot exceed 1 MiB. The current
  set is far below that. Over the budget the Operator reports a condition rather than truncating.
- Baking the chart in means a chart release requires an Operator release. `CHART_VERSIONS` carries
  several GitLab versions for this reason, and `SIPHON_CHART_VERSIONS` may have to as well.
- The Siphon chart ships a values schema, unlike `orbit-helm-charts`. A render
  already fails on a bad value, so the validating webhook ADR 26 describes is implementable for this
  resource without an upstream prerequisite.
