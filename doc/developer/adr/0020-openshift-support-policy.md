---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 20. OpenShift support policy
---

Date: 2026-07-08

Status: Accepted

Referred by [22. OpenShift and Kubernetes platform versions in CI](0022-openshift-and-kubernetes-platform-versions-in-ci.md)

## Context

The Distribution team needs to define how we support GitLab on various
OpenShift releases. Every OpenShift release ships with an underlying Kubernetes
version. This lets us derive OpenShift support from the Kubernetes support
policy instead of maintaining a separate version matrix.

## Decision

GitLab Operator supports every OpenShift version whose underlying Kubernetes
version is supported under the
[Kubernetes support policy](0019-k8s-support-policy.md).

## Consequences

- The supported OpenShift versions follow directly from the supported Kubernetes
  versions. This keeps the policy predictable and removes the need to track
  OpenShift versions separately.
