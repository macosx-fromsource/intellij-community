---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Opérateur GitLab
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

L'[opérateur GitLab](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator) est une méthode d'installation et de gestion qui suit le [modèle de l'opérateur Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

Utilisez l'opérateur GitLab pour exécuter GitLab dans [OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/) ou sur une autre plateforme compatible Kubernetes.

> [!note]
> L'opérateur GitLab présente des [limitations connues](#known-issues) et n'est adapté qu'à des scénarios spécifiques en production.

L'opérateur GitLab nécessite un [PostgreSQL](https://docs.gitlab.com/charts/advanced/external-db/), un [Redis](https://docs.gitlab.com/charts/advanced/external-redis/) et un [stockage d'objets](https://docs.gitlab.com/charts/advanced/external-object-storage/) externes.

Pour les déploiements en production, suivez les [architectures de référence cloud-native](https://docs.gitlab.com/administration/reference_architectures).

## Problèmes connus {#known-issues}

L'opérateur GitLab ne prend pas en charge :

- La gestion des instances existantes basées sur des charts Helm avec l'opérateur GitLab. La prise en charge des améliorations est proposée dans le [ticket de l'opérateur GitLab 1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567).
- Git via SSH avec les [routes OpenShift](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html). Pour plus d'informations, consultez la [documentation de l'opérateur GitLab sur les routes OpenShift](openshift_ingress.md#openshift-routes).
- [Identité de charge de travail GKE](https://docs.cloud.google.com/kubernetes-engine/docs/concepts/workload-identity) et les [comptes de service IAM](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html) pour authentifier les charges de travail auprès d'autres API cloud (telles que le stockage d'objets). Pour plus d'informations, consultez le [ticket de l'opérateur GitLab 1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737).
- Par défaut, l'opérateur GitLab met à niveau GitLab en utilisant la méthode zéro temps d'arrêt. Par conséquent, la version de GitLab et du chart GitLab doit être mise à jour une release mineure à la fois. Pour ignorer les versions mineures, vous pouvez [désactiver les mises à niveau sans interruption](gitlab_upgrades.md#upgrade-with-downtime), ce qui entraîne un temps d'arrêt lors de la mise à niveau.

L'opérateur GitLab présente toutes les autres limitations du chart GitLab. Il s'appuie sur le chart GitLab pour provisionner les ressources Kubernetes. Par conséquent, toute limitation dans le chart GitLab impacte l'opérateur GitLab. La suppression de la dépendance au chart GitLab de l'opérateur GitLab est proposée dans l'[epic 64 Cloud Native](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64).

## Installation {#installation}

Les instructions sur la façon d'installer l'opérateur GitLab sont disponibles dans notre [document d'installation](installation.md).

Nous répertorions les détails sur la façon dont nous utilisons les [contraintes de contexte de sécurité](security_context_constraints.md) dans leur document respectif.

Vous devez également connaître les [considérations relatives à l'accès SSH à Git](git_over_ssh.md), en particulier lors de l'utilisation d'OpenShift.

## Mise à niveau {#upgrading}

Pour savoir comment mettre à niveau l'opérateur GitLab ou une instance GitLab gérée par l'opérateur GitLab, consultez le fichier [mettre à niveau les instances GitLab avec l'opérateur GitLab](gitlab_upgrades.md).

## Sauvegarde et restauration {#backup-and-restore}

La documentation sur la [sauvegarde et la restauration](backup_and_restore.md) explique comment sauvegarder et restaurer une instance GitLab gérée par l'opérateur GitLab.

## Utilisation des images certifiées RedHat {#using-redhat-certified-images}

La documentation sur les [images certifiées RedHat](certified_images.md) explique comment demander à l'opérateur GitLab de déployer des images certifiées par RedHat.

## Outils pour les équipes de développement {#developer-tooling}

- [Guide des équipes de développement](developer/guide.md) :  décrit la structure du projet et la façon de contribuer.
- [Informations sur les versions et les releases](developer/releases.md) :  enregistre les notes concernant la gestion des versions et la publication de l'opérateur.
- [Décisions de conception](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/tree/master/doc/developer/adr) :  ce projet utilise des enregistrements de décisions d'architecture qui détaillent la structure, les fonctionnalités et l'implémentation des fonctionnalités de l'opérateur GitLab.

## Revues de merge request {#merge-request-reviews}

Les merge requests suivent généralement notre pratique standard qui exige 2 relecteurs. Dans un premier temps, un utilisateur qui n'est pas un chargé de maintenance examine la merge request et fournit des commentaires à l'auteur pour l'aider à améliorer/corriger le changement proposé. Une fois que l'auteur a effectué les mises à jour nécessaires et que le relecteur approuve la merge request, nous demandons une revue à l'un des chargés de maintenance.

Cette approche offre des opportunités d'apprentissage pour les relecteurs moins expérimentés. La première revue traite la plupart des problèmes d'une merge request avant la revue finale. Les projets à fort volume connaissent souvent des goulots d'étranglement dus au nombre de tâches attribuées aux chargés de maintenance, et ce premier passage permet de réduire ce nombre.

### Exceptions pour une seule approbation {#one-approval-only-exceptions}

Dans certains cas, nous autorisons la fusion des merge requests avec une seule approbation.

#### Mises à jour des modules Go {#go-modules-updates}

> [!note]
> Cela n'est pertinent que pour les membres de l'équipe GitLab du groupe propriétaire de ce projet.

Si vous êtes membre de l'équipe propriétaire de ce projet, vous avez reçu des droits d'approbation CODEOWNERS sur les fichiers `go.mod` et `go.sum`. Si la merge request ne modifie que ces fichiers, vous devriez être en mesure d'approuver la merge request et de la fusionner, même si vous n'êtes pas chargé de maintenance. Cette approche a été mise en place pour réduire le travail lié aux revues des chargés de maintenance et améliorer l'efficacité des mises à jour de dépendances, étant donné que l'équipe a évalué que les mises à jour des modules Go présentaient un risque très faible. Ainsi, si vous êtes à l'aise avec Go, que le changement vous semble correct et que vous disposez d'un pipeline entièrement vert sur la merge request, n'hésitez pas à l'approuver et à la fusionner immédiatement.

Cependant, si vous n'êtes pas à l'aise avec le code Go, ou que vous souhaitez un second avis pour toute autre raison, n'hésitez pas à transférer la merge request à un chargé de maintenance pour que celui-ci effectue une seconde revue.
