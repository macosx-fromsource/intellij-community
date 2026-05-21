---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Contraintes de contexte de sécurité
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

## Vue d'ensemble {#overview}

Dans OpenShift, les pods reçoivent des autorisations basées sur leurs contraintes de contexte de sécurité. Les contraintes de contexte de sécurité, souvent abrégées **SCC**, simplifient le mécanisme de contrôle d'accès basé sur les rôles pour une utilisation dans des déploiements à grande échelle. [Les administrateurs peuvent consulter la documentation en amont pour mieux comprendre le fonctionnement des contraintes de contexte de sécurité et leur importance dans OpenShift](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html).

Les administrateurs peuvent également consulter les ressources suivantes :

1. [Gestion des contraintes de contexte de sécurité dans OpenShift](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [Guide relatif à OpenShift et aux identifiants utilisateur](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## Contraintes de contexte de sécurité au sein du déploiement GitLab {#security-context-constraints-within-the-gitlab-deployment}

Le déploiement `gitlab-controller-manager` crée et gère le pod contenant les processus de l'**opérateur GitLab**. Celui-ci et tout autre pod qu'il crée et gère s'exécutent avec la contrainte de contexte de sécurité **restricted**.

L'**opérateur GitLab** utilise un compte de service avec des autorisations étendues qui lui permettent de gérer toutes les ressources requises par l'application GitLab.

L'**opérateur GitLab** gère les services qui composent GitLab cloud-native. Il arrête activement et remplace les pods qui ne sont pas conformes à l'identifiant utilisateur (UID) spécifié par l'**opérateur GitLab**. Ce mécanisme applique le principe du moindre privilège.

### Définitions de ressources personnalisées de l'application GitLab {#gitlab-application-custom-resource-definitions}

Les pods déployés par l'opérateur GitLab pour satisfaire les ressources personnalisées GitLab utilisent la contrainte de contexte de sécurité **nonroot-v2**. Les contraintes de contexte de sécurité pour les opérateurs et ressources tiers sont [traitées dans la section suivante](#third-party-resource-definitions).

Le compte de service `gitlab-app-nonroot` n'a aucun privilège accordé et existe uniquement pour lier les contraintes de contexte de sécurité **nonroot-v2** aux pods de l'application GitLab.

Les contraintes de contexte de sécurité seront renforcées dans les futures releases à mesure que l'ensemble des comportements de lecture/écriture de l'application GitLab seront validés dans le modèle de sécurité OpenShift.

> [!note] 
> Les administrateurs qui passent à GitLab cloud-native depuis une installation de paquet Linux doivent noter que les tâches d'installation de paquet Linux effectuées avec `sudo` sont gérées par OpenShift et le moteur Kubernetes sous-jacent. Les pods sont des services individuels qui, dans une installation de paquet Linux, abandonnent les privilèges pour s'exécuter en tant qu'utilisateur spécifique à l'application. L'**opérateur GitLab** va [interrompre tout pod qui ne fonctionne pas avec l'identifiant utilisateur attendu](#security-context-constraints-within-the-gitlab-deployment).

### Définitions de ressources tierces {#third-party-resource-definitions}

### Contrôleur Ingress {#ingress-controller}

GitLab recommande et teste les déploiements utilisant `nginx-ingress-controller` lors du déploiement de GitLab cloud-native. Il utilise sa propre [contrainte de contexte de sécurité `nginx-ingress-scc`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml).

Si vous sélectionnez un contrôleur Ingress alternatif, veuillez consulter la documentation pertinente pour en savoir plus sur ses contraintes de contexte de sécurité.

### Chiffrement SSL {#ssl-encryption}

L'opérateur GitLab nécessite que [`cert-manager`](https://cert-manager.io/docs/releases/) soit installé séparément comme prérequis. L'opérateur GitLab configure les émetteurs et certificats `cert-manager` pour gérer TLS dans l'ensemble de l'application GitLab. `cert-manager` ne définit aucune contrainte de contexte de sécurité directement ; par conséquent, OpenShift appliquera la contrainte de contexte de sécurité **restricted** par défaut.
