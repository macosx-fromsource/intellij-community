---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 8. Operator scoped to namespace by default
---

Date: 2020-08-03
Updated: 2021-09-07

## Status

Accepted

## Context

Operator RBAC roles are currently scoped to the entire cluster, but true cluster-scoped support is not ready yet. For example, attempting
to deploy a CR to a namespace other than the Operator will not work because the GitLab service accounts do not exist in that
namespace.

We've seen failures in CI (<https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/209>) related to Operators attempting
to reconcile objects in namespaces other than their own. This is particularly undesirable in CI where each namespace is intended
to be isolated from other releases.

## Decision

Ultimately we want the Operator to be able to resolve GitLab CRs cluster-wide, in any namespace. Users should easily be able to toggle between cluster-scoped and namespace-scoped behavior.

The Operator's current RBAC permissions make it cluster-scoped (via a ClusterRole), but it is configured to only resolve GitLab CRs that are in the same namespace as the Operator itself using the `WATCH_NAMESPACE` variable.

Cluster-scope can be configured by disabling the `WATCH_NAMESPACE` patch that we provide. This is currently experimental.

## Consequences

The Operator is namespace-scoped by default. CI pipelines will pass more reliably as this change ensures that Operators only reconcile objects within their own namespace.

We are investigating support for cluster-scope in <https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/100> with the intention of allowing the Operator to resolve GitLab objects in any namespace. We will update this ADR upon resolving that issue.
