---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Prendre en charge Git via SSH
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

Ce document fournit des instructions de configuration pour Git via SSH sur différents environnements/plateformes.

## Vue d'ensemble {#overview}

Le [chart Helm GitLab Shell](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/) fournit un serveur SSH configuré pour l'accès SSH Git à GitLab. Ce composant doit être exposé en dehors du cluster sur le port `22`.

L'opérateur GitLab déploie `gitlab-shell` lorsque `gitlab.gitlab-shell.enabled` est défini sur `true` (paramètre par défaut).

Pour résumer les exigences en fonction de la plateforme cible :

| Avez-vous besoin de Git via SSH ? | Kubernetes                                                                                                    | OpenShift |
|------------------------------|---------------------------------------------------------------------------------------------------------------|-----------|
| Non                           | Vous devez utiliser l'un des fournisseurs NGINX Ingress ci-dessous (Kubernetes ne dispose pas d'un fournisseur Ingress intégré). | Vous n'avez pas besoin des fournisseurs Ingress ci-dessous, vous pouvez utiliser les Routes intégrées comme fournisseur Ingress. |
| Oui                          | Vous devez utiliser l'un des fournisseurs NGINX Ingress ci-dessous (Kubernetes ne dispose pas d'un fournisseur Ingress intégré). | Vous devez utiliser l'un des fournisseurs Ingress ci-dessous, les Routes ne prennent pas en charge l'exposition du port `22`. |

## Fournisseurs Ingress {#ingress-providers}

Vous trouverez ci-dessous une liste de fournisseurs Ingress accompagnée de notes pertinentes et de détails spécifiques à chaque plateforme.

### Chart Helm NGINX-Ingress {#nginx-ingress-helm-chart}

GitLab maintient un [chart `NGINX-ingress` dupliqué](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork) qui peut être utilisé pour déployer des ressources NGINX modifiées pour prendre en charge Git via SSH « prêt à l'emploi ».

Il s'agit de la configuration par défaut lors de l'utilisation de l'opérateur GitLab, et elle est contrôlée par `nginx-ingress.enabled={true,false}` dans la ressource personnalisée GitLab. Lorsqu'il est défini sur `false`, vous pouvez utiliser une [instance NGINX externe](https://docs.gitlab.com/charts/advanced/external-nginx/).

Ce fournisseur Ingress peut être utilisé à la fois sur Kubernetes et OpenShift.

De plus amples informations sur les options d'installation du fournisseur NGINX Ingress sont disponibles dans notre [documentation d'installation](installation.md#ingress-controller).

### Opérateur NGINX Ingress {#nginx-ingress-operator}

En tant qu'alternative à la duplication du chart Helm NGINX-Ingress intégré, l'[opérateur NGINX Ingress](https://github.com/nginxinc/nginx-ingress-operator) peut être utilisé pour exposer `gitlab-shell`.

Cette option comporte certaines restrictions :

- Les définitions de ressources personnalisées TransportServer/GlobalConfiguration de NGINX Inc. sont considérées comme une fonctionnalité en aperçu, et il est recommandé de faire preuve de prudence pour une utilisation en production.
- L'opérateur NGINX Inc. est encore relativement récent (version 0.3.0 seulement). Il ne contient pas autant d'options de configuration que les charts Helm plus matures, quelle que soit leur variante.
- Cette option nécessite toujours d'exposer manuellement le port `22` sur le service NGINX (pas configurable dans le CR NGINXIngressController).

Des recherches plus approfondies sont consignées dans [\#58](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/58#note_585883916).

### Routes OpenShift {#openshift-routes}

Les [Routes](https://docs.openshift.com/container-platform/3.4/architecture/core_concepts/routes.html) OpenShift sont un composant intégré aux clusters OpenShift. Elles sont l'équivalent OpenShift des [Ingresses Kubernetes](https://kubernetes.io/docs/concepts/services-networking/ingress/).

Lors du déploiement sur OpenShift, vous pouvez définir `nginx-ingress.enabled=false` dans la ressource personnalisée GitLab et autoriser les Routes OpenShift à contrôler le flux du trafic externe. Lorsque l'opérateur GitLab réconcilie les objets Ingress, OpenShift crée automatiquement un objet Route équivalent qui correspond au domaine de base du cluster.

Notez que les Routes OpenShift ne prennent pas en charge l'exposition du trafic TCP (SSH sur le port `22`), et ne peuvent donc pas être utilisées pour Git via SSH via `gitlab-shell`.

## Éléments à prendre en compte {#considerations}

Vous trouverez ci-dessous des éléments à prendre en compte lors de l'utilisation d'Ingress.

### Utilisation d'un fournisseur Ingress tiers dans OpenShift {#using-a-third-party-ingress-provider-in-openshift}

Lors de l'utilisation d'un contrôleur Ingress tiers sur OpenShift, le contrôleur Ingress OpenShift peut entrer en conflit avec le contrôleur Ingress tiers dans certains cas.

Un exemple : le contrôleur NGINX Ingress définira une `ADDRESS` Ingress sur l'adresse IP externe du service NGINX, puis le contrôleur Ingress OpenShift la remplacera par le domaine de base du cluster. Cela peut entrer en conflit avec la configuration DNS, en particulier lors de l'utilisation d'un service comme [external-dns](https://github.com/kubernetes-sigs/external-dns) qui repose sur le fait que l'Ingress dispose d'une adresse IP pour pouvoir créer des enregistrements A afin de mapper l'URL vers ce service NGINX spécifique. C'est le cas dans l'environnement CI de l'opérateur GitLab.

Pour contourner ce problème, nous [corrigeons le contrôleur Ingress OpenShift](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26) pour qu'il gère uniquement les espaces de nommage spécifiques à OpenShift afin de garantir que les Ingresses que nous créons dans les espaces de nommage spécifiques à GitLab ne sont pas modifiés de manière indésirable.
