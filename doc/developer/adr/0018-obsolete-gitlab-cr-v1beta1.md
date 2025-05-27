---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 18. Obsolete GitLab resource v1beta1
---

Date: 2023-11-29

Status: Accepted

## Context

GitLab resource version `v1beta1` is heavily based on GitLab Chart configuration
model. But the new version of GitLab resource introduces a new configuration
style and uses a [structured specification](0017-structured-spec-for-gitlab-cr.md).
It is part of the attempt to decouple GitLab Operator from GitLab Chart.

## Decision

GitLab resource version will be obsolete and we will not support its
automatic conversion to the new version. However, GitLab Operator supports it
along with the new version until the structured specification is generally available.

## Consequences

Since there is no [conversion](https://book.kubebuilder.io/multiversion-tutorial/conversion)
between the `v1beta1` and the new version, users need to manually migrate their
existing GitLab instances that are managed by GitLab Operator. We may have to
provide some sort of tooling to assist with the migration.
