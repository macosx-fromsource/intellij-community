---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Backup and Restore GitLab
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

This document outlines how to backup and restore your GitLab instance using Toolbox.

## General backup and restore guidance

The Operator deploys the [Toolbox chart](https://docs.gitlab.com/charts/charts/gitlab/toolbox/), meaning
our existing documentation on how to [backup and restore a GitLab instance](https://docs.gitlab.com/charts/backup-restore/)
is applicable to the Operator.

## Migration between Helm-based and Operator-based installations

Backups created in a Helm-based installation can typically be restored in an Operator-based installation,
and vice-versa.

These flows will be tested more extensively throughout the development of the Operator. See
[issue #320](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/320) for more information.

Note that environments that leverage external services for stateful components like PostgreSQL and Gitaly
are typically easier to migrate.
