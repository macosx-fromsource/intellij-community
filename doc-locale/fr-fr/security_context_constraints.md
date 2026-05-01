---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Contraintes de contexte de sécurité
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

## Vue d'ensemble {#overview}

Les pods dans OpenShift reçoivent des autorisations basées sur leurs contraintes
de contexte de sécurité. Les contraintes de contexte de sécurité, souvent abrégées **SCC**,
simplifient le mécanisme de contrôle d'accès basé sur les rôles pour une utilisation dans des déploiements
à grande échelle. [Les administrateurs peuvent consulter la documentation officielle pour mieux comprendre le fonctionnement des contraintes de contexte de sécurité et leur rôle dans OpenShift](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html)

Les administrateurs peuvent également consulter les ressources suivantes :

1. [Managing Security Context Constraints in OpenShift](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [A Guide to OpenShift and UIDs](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## Contraintes de contexte de sécurité au sein du déploiement GitLab {#security-context-constraints-within-the-gitlab-deployment}

Le déploiement `gitlab-controller-manager` crée et gère le pod
contenant les processus de l'**Opérateur**. Ce pod, ainsi que tout autre pod qu'il crée et
gère, s'exécutent avec la contrainte de contexte de sécurité **restricted**.

L'**Opérateur** utilise un ServiceAccount doté d'autorisations étendues lui permettant
de gérer toutes les ressources requises par l'application GitLab.

L'**Opérateur** gère les services composant Cloud Native
GitLab. Il arrête et remplace activement les pods qui ne sont pas conformes à
l'UID spécifié par l'**Opérateur**. Ce mécanisme applique le principe du
moindre privilège.

### Définitions de ressources personnalisées de l'application GitLab {#gitlab-application-custom-resource-definitions}

Les pods déployés par l'Opérateur pour satisfaire les ressources personnalisées GitLab utilisent la
contrainte de contexte de sécurité **non-root-v2**. Les contraintes de contexte de sécurité pour
les opérateurs et ressources tiers sont [traitées dans la section suivante](#third-party-resource-definitions).

Le ServiceAccount `gitlab-app-nonroot` ne dispose d'aucun privilège accordé et existe uniquement
pour lier les contraintes de contexte de sécurité **nonroot-v2** aux pods de l'application GitLab.

Les contraintes de contexte de sécurité seront renforcées dans les versions futures, au fur et à mesure que
les comportements complets de lecture/écriture de l'application GitLab seront validés dans
le modèle de sécurité OpenShift.

> [!note]
> Les administrateurs qui migrent vers Cloud Native GitLab depuis une installation par package Linux doivent noter que
> les tâches d'installation par package Linux effectuées avec `sudo` sont gérées par OpenShift et le
> moteur Kubernetes sous-jacent. Les pods sont des services individuels qui, dans une installation par package Linux,
> abandonnent les privilèges pour s'exécuter en tant qu'utilisateur spécifique à l'application. L'
> **Opérateur** [arrêtera tout pod qui ne fonctionne pas avec l'UID attendu](#security-context-constraints-within-the-gitlab-deployment).

### Définitions de ressources tierces {#third-party-resource-definitions}

### Contrôleur Ingress {#ingress-controller}

GitLab recommande et teste les déploiements en utilisant le
`nginx-ingress-controller` lors du déploiement de Cloud Native GitLab. Il utilise sa
propre [contrainte de contexte de sécurité `nginx-ingress-scc`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml).

Si vous optez pour un contrôleur Ingress alternatif, veuillez consulter la documentation
correspondante pour en savoir plus sur ses contraintes de contexte de sécurité.

### Chiffrement SSL {#ssl-encryption}

GitLab Operator requiert l'installation préalable de [`cert-manager`](https://cert-manager.io/docs/releases/)
en tant que prérequis. GitLab Operator configure les émetteurs et certificats `cert-manager`
pour gérer le TLS au sein de l'application GitLab.
`cert-manager` ne définit aucune contrainte de contexte de sécurité directement ; par conséquent,
OpenShift applique par défaut la contrainte de contexte de sécurité **restricted**.
