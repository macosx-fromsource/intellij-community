---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 4. Integration of the GitLab chart
---

Date: 2020-11-16

## Status

Accepted

## Context

Leveraging the GitLab chart greatly accelerates the progression of the GitLab Operator by capturing the
objects and logic from the charts.

## Decision

The Operator will render the [GitLab chart](https://gitlab.com/gitlab-org/charts/gitlab) using the `values`
field from the GitLab CR to create a template similar to the output of `helm template`.

The Operator will query this template for objects to deploy based on the configuration provided in the
CR values.

## Consequences

This means the Operator is effectively a wrapper around the GitLab chart,
with additional capabilities aiming to fulfill the
[Operator maturity model](https://docs.openshift.com/container-platform/4.1/applications/operators/olm-what-operators-are.html#olm-maturity-model_olm-what-operators-are).
