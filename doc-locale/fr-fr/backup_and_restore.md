---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Sauvegarde et restauration de GitLab
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

GitLab Operator déploie le [chart Toolbox](https://docs.gitlab.com/charts/charts/gitlab/toolbox/).
Pour sauvegarder et restaurer votre instance GitLab à l'aide de Toolbox, consultez la page [sauvegarde et restauration de GitLab](https://docs.gitlab.com/charts/backup-restore/).

## Migrer entre des installations basées sur Helm et des installations basées sur Operator {#migrate-between-helm-based-and-operator-based-installations}

Vous pouvez créer de nouvelles instances basées sur des charts Helm à partir de sauvegardes d'instances basées sur Operator, et de nouvelles instances basées sur Operator à partir de sauvegardes d'instances basées sur des charts Helm.

Les environnements qui utilisent des services externes pour les composants avec état, tels que PostgreSQL et Gitaly, sont généralement plus faciles à migrer.
