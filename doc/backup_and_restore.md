---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Backup and Restore GitLab
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

GitLab Operator deploys the [Toolbox chart](https://docs.gitlab.com/charts/charts/gitlab/toolbox/).
To back up and restore your GitLab instance using Toolbox, see [backup and restore GitLab](https://docs.gitlab.com/charts/backup-restore/).

## Migration between Helm-based and Operator-based installations

You can restore backups between Helm-based and Operator-based installations.

Environments that use external services for stateful components like PostgreSQL and Gitaly
are typically easier to migrate.
