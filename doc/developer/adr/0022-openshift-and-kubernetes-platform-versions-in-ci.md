---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 22. OpenShift and Kubernetes platform versions in CI
---

Date: 2026-07-08

## Status

Accepted

Refers [20. OpenShift support policy](0020-openshift-support-policy.md)

Refers [19. Kubernetes support policy](0019-k8s-support-policy.md)

Refers [14. Supported OpenShift versions](0014-supported-openshift-versions.md)

## Context

GitLab Operator supports multiple versions of Kubernetes and OpenShift within each Operator release.
At runtime, the Operator has little OpenShift-specific behavior. The main difference is security
context constraint handling. Because the runtime behavior is nearly identical across OpenShift
versions, testing against a single OpenShift version is enough to gain confidence in the Operator
bundle for OpenShift.

## Decision

GitLab Operator end-to-end tests run:

- On every supported Kubernetes version.
- On a single OpenShift version, to validate the Operator bundle on OpenShift.

## Consequences

1. We must maintain CI end-to-end testing for every supported Kubernetes version.
1. Each time a supported version changes, the CI definitions and clusters must be adjusted.
