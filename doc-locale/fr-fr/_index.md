---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Operator
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

[GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator) est une méthode d'installation et de gestion qui suit le
[modèle Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

Utilisez GitLab Operator pour exécuter GitLab dans
[OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/) ou sur
une autre plateforme compatible Kubernetes.

> [!note]
> GitLab Operator présente des [limitations connues](#known-issues) et n'est adapté qu'à des scénarios spécifiques en production.

<!-- This warning block is duplicated in doc/installation.md. Changes should be reflected in both locations. -->

> [!warning]
> Les valeurs par défaut de la ressource personnalisée GitLab **ne sont pas destinées à une utilisation en production**.
> Avec ces valeurs, GitLab Operator crée une instance GitLab où tous les services, y compris les données persistantes,
> sont déployés dans un cluster Kubernetes, ce qui **n'est pas adapté aux charges de travail en production**.
> Pour les déploiements en production, vous **devez** suivre les [architectures de référence Cloud Native Hybrid](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid).
> GitLab ne prend pas en charge les problèmes liés à PostgreSQL, Redis, Gitaly, Praefect ou MinIO déployés au sein d'un cluster Kubernetes.

## Problèmes connus {#known-issues}

GitLab Operator ne prend pas en charge :

- La gestion des instances existantes basées sur des charts Helm avec GitLab Operator. La prise en charge des améliorations est proposée dans
  [GitLab Operator issue 1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567).
- Git via SSH avec les [routes OpenShift](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html).
  Pour plus d'informations, consultez la [documentation GitLab Operator sur les routes OpenShift](openshift_ingress.md#openshift-routes).
- [L'identité de charge de travail GKE](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity) et les [comptes de service IAM](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html) pour authentifier les charges de travail auprès d'autres API cloud (telles que le stockage d'objets).
  Pour plus d'informations, consultez [GitLab Operator issue 1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737).
- Par défaut, l'Operator met à niveau GitLab selon la méthode sans interruption de service. Par conséquent, la version de GitLab et du chart GitLab
  doit être mise à jour une version mineure à la fois. Pour ignorer des versions mineures, vous pouvez
  [désactiver les mises à niveau sans interruption de service](gitlab_upgrades.md#upgrade-with-downtime), ce qui entraîne une interruption de service pendant
  la mise à niveau.

GitLab Operator présente toutes les autres limitations du chart GitLab. GitLab Operator s'appuie sur le chart GitLab pour provisionner les ressources Kubernetes. Par conséquent, toute limitation
du chart GitLab a un impact sur GitLab Operator. La suppression de la dépendance au chart GitLab dans GitLab Operator est proposée dans
[Cloud Native epic 64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64).

## Installation {#installation}

Les instructions d'installation de GitLab Operator sont disponibles dans notre [document d'installation](installation.md).

Nous détaillons la façon dont nous utilisons les
[Security Context Constraints](security_context_constraints.md) dans leur document respectif.

Vous devez également prendre connaissance des [considérations relatives à l'accès SSH à Git](git_over_ssh.md), en particulier
lors de l'utilisation d'OpenShift.

## Mise à niveau {#upgrading}

Pour savoir comment mettre à niveau GitLab Operator ou une instance GitLab gérée par GitLab Operator,
consultez [Mettre à niveau des instances GitLab avec GitLab Operator](gitlab_upgrades.md).

## Sauvegarde et restauration {#backup-and-restore}

La documentation [Sauvegarde et restauration](backup_and_restore.md) explique comment sauvegarder et restaurer une instance GitLab gérée par l'Operator.

## Utilisation des images certifiées RedHat {#using-redhat-certified-images}

La documentation sur les [images certifiées RedHat](certified_images.md) explique comment configurer GitLab Operator
pour déployer des images certifiées par RedHat.

## Outils pour les développeurs {#developer-tooling}

- [Guide du développeur](developer/guide.md) : présente la structure du projet et les modalités de contribution.
- [Informations sur le versionnement et les releases](developer/releases.md) : consigne les notes relatives au versionnement et à la publication de l'Operator.
- [Décisions de conception](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr) : ce projet utilise des Architecture Decision Records, qui détaillent la structure, les fonctionnalités et l'implémentation des fonctionnalités de cet Operator.

## Revues de merge requests {#merge-request-reviews}

Les merge requests (MR) suivent généralement notre pratique standard qui requiert 2 réviseurs. Dans un premier temps, un réviseur non-mainteneur
examine la MR et formule des commentaires à l'intention de l'auteur pour l'aider à améliorer ou corriger la modification proposée. Une fois que
l'auteur a effectué les mises à jour nécessaires et que le réviseur a approuvé la MR, nous sollicitons la revue d'un
des mainteneurs.

Cette approche offre des opportunités d'apprentissage aux réviseurs moins expérimentés. La
première revue permet de traiter la plupart des problèmes d'une MR avant la revue finale. Les projets à
fort volume connaissent souvent des goulots d'étranglement en raison de la charge des mainteneurs, et cette première passe
contribue à alléger leur charge de travail.

### Exceptions à l'approbation unique {#one-approval-only-exceptions}

Dans certains cas, nous autorisons la fusion des MR avec une seule approbation.

#### Mises à jour des modules Go {#go-modules-updates}

> [!note]
> Ceci ne concerne que les membres de l'équipe GitLab du groupe propriétaire de ce projet.

Si vous êtes membre de l'équipe propriétaire de ce projet, vous disposez des droits d'approbation CODEOWNERS sur les fichiers
`go.mod` et `go.sum`. Si la MR ne modifie que ces fichiers, vous devriez pouvoir approuver la MR
et la fusionner, même si vous n'êtes pas mainteneur. Cette mesure a été mise en place pour réduire la charge de revue des mainteneurs
et améliorer l'efficacité des mises à jour des dépendances, l'équipe ayant évalué que les mises à jour des modules Go présentent un
risque très faible. Ainsi, si vous maîtrisez Go, que la modification vous semble correcte et que vous disposez d'un pipeline
entièrement vert sur la MR, n'hésitez pas à approuver et fusionner directement.

Cela dit, si vous n'êtes pas à l'aise avec le code Go, ou pour toute autre raison souhaitez un second avis,
vous êtes également encouragé à transmettre la MR à un mainteneur pour une seconde revue.
