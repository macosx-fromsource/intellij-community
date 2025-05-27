---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 13. Stable branch policy
---

Date: 2021-09-16

## Status

Accepted

## Context

Release of a GitLab version normally include a stable branch corresponding to it
being created from a known good commit from master/main branch, and a Git tag
being created from this stable branch. When new patch releases need to be done,
the commits are cherry-picked into these stable branches and tagged.

We will implement similar for Operator releases, following the implementation of [release branches with GitLab flow](https://docs.gitlab.com/topics/gitlab_flow/#release-branches-with-gitlab-flow) used in our other projects.

## Decision

1. For releasing a version X.Y.0 of GitLab Operator, a stable branch with the
   naming format `X-Y-stable` will be created from the master branch.
1. For patch releases X.Y.Z (where Z != 0), the new commits will be
   cherry-picked into existing `X-Y-stable` stable branch.
1. [Git tag needed for the release](0011-operator-release-process.md) will be
   created from this stable branch.

## Consequences

Possibility of commits not getting cleanly applied to the stable branch because
of other changes made to the master branch since stable branch was cut. But,
this is the case with every other project and isn't anything we aren't already
familiar with
