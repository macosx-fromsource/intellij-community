---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 22. OpenShift and Kubernetes platform versions in CI
---

Date: 2024-04-25

## Status

Accepted

Refers [20. OpenShift support policy](0020-openshift-support-policy.md)

Refers [19. Kubernetes support policy](0019-k8s-support-policy.md)

Refers [14. Supported OpenShift versions](0014-supported-openshift-versions.md)

## Context

Typically we support multiple versions (>2) of Kubernetes or OpenShift within each Operator release. From past discussions
it was deemed feasible to test only against oldest supported version and newest supported versions for each platform to gain
sufficient confidence that software will run on any platform "in between". There's also a need to cut down on resource utilization
correspondingly to cut down operational costs.

## Decision

We will maintain OpenShift instances covering range of supported versions of platform.

## Consequences

1. At all time we should have at 2 CI clusters that satisfy `min-max` criteria.
1. Each time we raise mimimal version we may need to provision new cluster
1. There is a chance that cluster version may need to be provisioned twice - once as a `max` version and second time as `min` version.
