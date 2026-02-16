---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 17. Structured Specification of GitLab custom resource
---

Date: 2023-11-28

Status: Proposed

## Context

The specification of version `v1beta1` of GitLab resource accepts GitLab chart
version and a free-format set of values. This is because GitLab Operator uses
GitLab chart and adopts its configuration model.

### A working definition for _structured specification_

A structured specification does not accept an arbitrary set of keys and values.
The property names and types are pre-defined and conform to [Kubernetes API convention](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md).
The specification reuses available [Kubernetes API types](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/)
where possible to improve user experience by utilizing well-known types.

## Decision

We will abandon the GitLab chart configuration model and Helm-style values.
Instead we adopt a structured specification for GitLab custom resource and
provide a new version for GitLab resource. This will be done as a new CRD.

## Consequences

A structured resource specification provides better semantic configuration. It
helps us to introduce a configuration model that is not bound by Helm values
hierarchy and its limitations. Overall it will improve the user experience.
