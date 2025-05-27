---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 7. Dependency on other external components
---

Date: 2020-12-01

## Status

Accepted

## Context

Operator depends on external components. We need to define how are we going to approach such external dependencies moving forward.

## Decision

The Helm charts include a chart for `cert-manager`.
In contrast, the Operator does not deploy this component and therefore requires it to be installed separately following advise from [best practices](https://sdk.operatorframework.io/docs/best-practices/best-practices/#development):

> An Operator should manage a single type of application, essentially following the UNIX principle: do one thing and do it well.

## Consequences

This keeps the codebase focused on GitLab functionality, and ensures that
the Operator is not tied to specific certificate managers.
