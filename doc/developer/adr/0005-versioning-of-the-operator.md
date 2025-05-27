---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 5. Versioning of the operator
---

Date: 2020-11-03

## Status

Accepted

Related [6. GitLab application versions supported](0006-gitlab-application-versions-supported.md)

Relates to [9. version tagging](0009-version-tagging.md)

## Context

The intention is to version the operator matching the version number as the
GitLab application and be on the same monthly release schedule.
There is a need tightly control the SA's of the application, to reduce the impact
for the need of AnyUID. There are several hurdles to overcome and achieve matching
the version numbers.

- Operator needs to support a greater feature parity with GitLab Helm chart
- Come to consensus on single operator or splitting into application and
  Runner operators. Issue: <https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/30>
- Proper handling of AnyUID in GitLab application containers. Issue:
  <https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/11>
- Decide on the level of namespace scoping that the operator will be capable of.
  Issue: <https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/31>

## Decision

The version of the operator shall follow semantic versioning during development
until the feature set is sufficient to support the GitLab application
lifecycle through the operator. At which point the version will be bumped
to be in sync with the GitLab application version.

Further information can be found in <https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/5>

## Consequences

We will follow outlined decision
