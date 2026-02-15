---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 9. version tagging
---

Date: 2021-08-09

## Status

Accepted

Relates to [5. Versioning of the operator](0005-versioning-of-the-operator.md)

## Context

Repo tagging direcly affects our releases as we derive release version from tags. In the past
there was a practice to prefix version tags with `v`.

Issue has been brought up in general terms at Distribution Team's weekly meeting on Aug 09, 2021.

## Decision

Use "plain version" as a tag bypassing `v` prefix:

```shell
# instead of: git tag 'v0.1.25'
git tag '0.1.25'
```

## Consequences

Plain version tags will allow for easy and consistent automation of release and packaging processes.
