---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Sauvegarder et restaurer GitLab
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

L'opérateur GitLab déploie le [chart Toolbox](https://docs.gitlab.com/charts/charts/gitlab/toolbox/). Pour sauvegarder et restaurer votre instance GitLab à l'aide de Toolbox, consultez la page [sauvegarder et restaurer GitLab](https://docs.gitlab.com/charts/backup-restore/).

## Migrer entre les installations basées sur Helm et les installations basées sur l'opérateur GitLab {#migrate-between-helm-based-and-operator-based-installations}

Vous pouvez créer de nouvelles instances basées sur les charts Helm à partir de sauvegardes d'instances basées sur l'opérateur GitLab, et de nouvelles instances basées sur l'opérateur GitLab à partir de sauvegardes d'instances basées sur les charts Helm.

Les environnements qui utilisent des services externes pour les composants stateful tels que PostgreSQL et Gitaly sont généralement plus faciles à migrer.
