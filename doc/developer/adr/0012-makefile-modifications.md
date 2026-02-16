---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 12. Makefile modifications
---

Date: 2021-07-27

## Status

Accepted

## Context

`operator-sdk` have created `Makefile` as part of scaffolding. Throughtout development cycle for operator we needed to modify
behavior of existing `Makefile`. One of the options could be maintenance of separate `Makefile.gitlab` to isolate scaffolding code from our own changes.

See also discussion in [merge request 181](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/181#note_635709947)

## Decision

It has been decided to maintain single `Makefile` with all of the original scaffolding and our changes for the time being for simplicity.

## Consequences

We will rely on Git history to resolve potential future conflicts (perhaps as part of operator-sdk upgrade)
