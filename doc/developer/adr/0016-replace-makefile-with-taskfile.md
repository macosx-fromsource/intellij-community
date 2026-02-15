---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 16. CI pipelines in dev
---

Date: 2022-05-23

## Status

Accepted

## Context

[Taskfile.dev](https://taskfile.dev) offers an alternative to a Makefile, focusing on readability by using YAML. This is a good
candidate for the GitLab Operator project given the extensive use of YAML in Kubernetes projects.

Our Makefile has become a bit difficult to parse as it has gotten more complex, which makes debugging and contributing more difficult.

[GitHub repository](https://github.com/go-task/task)

## Decision

Makefile will be replaced with a Taskfile, and calls to `make` will be replaced with calls to `task`.

## Consequences

1. Updates and additions to scripting will be easier to complete, test, and review.
1. Available tasks can be easily viewed using `task --list`.
