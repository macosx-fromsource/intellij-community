---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Prendre en charge Git via SSH
---

{{< details >}}

- Édition : version gratuite, GitLab Premium, GitLab Ultimate
- Offre : GitLab Self-Managed

{{< /details >}}

Le [chart Helm GitLab Shell](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/) fournit un serveur SSH configuré pour l'accès SSH Git à GitLab. Ce composant doit être exposé en dehors du cluster sur le port `22`.

L'opérateur GitLab déploie `gitlab-shell` lorsque `gitlab.gitlab-shell.enabled` est défini sur `true` (paramètre par défaut).

Pour exposer Git via SSH, utilisez l'une des méthodes suivantes :

| Méthode | Kubernetes | OpenShift | Remarques |
|:-----------------|:-----------|:---------------|:------|
| Gateway API | Prise en charge | Prise en charge | Approche moderne recommandée utilisant le standard Kubernetes Gateway API avec TCPRoute. Envoy Gateway est recommandé. [D'autres fournisseurs](https://docs.gitlab.com/charts/advanced/gateway-api/#using-an-external-gateway-api-provider) peuvent fonctionner s'ils répondent aux exigences. |
| NGINX Ingress | Déprécié | Déprécié | Approche traditionnelle (nécessite l'exposition du port 22). Des [contrôleurs NGINX externes](https://docs.gitlab.com/charts/advanced/external-ingress/) peuvent être utilisés à la place du contrôleur intégré. |
| Routes OpenShift | N/A | Pas de prise en charge SSH | Les Routes ne prennent pas en charge le trafic TCP (port 22). |

## Gateway API avec Envoy Gateway {#gateway-api-with-envoy-gateway}

GitLab peut être exposé via [Gateway API](https://gateway-api.sigs.k8s.io/) plutôt qu'avec des ressources Ingress traditionnelles.
Cette méthode est recommandée pour les nouveaux déploiements, car elle prend en charge nativement le routage TCP pour Git via SSH grâce aux ressources `TCPRoute`.

Prérequis :

- GitLab Operator 2.10 ou version ultérieure.
- GitLab chart 9.7 ou version ultérieure.

Gateway API fonctionne sur les clusters Kubernetes et OpenShift. Pour des instructions de configuration détaillées et les prérequis,
consultez la [documentation Gateway API et Envoy Gateway](gatewayapi.md).

Lorsque Gateway API est activé avec `global.gatewayApi.enabled: true`, `gitlab-shell` est automatiquement exposé via une ressource `TCPRoute` qui achemine le trafic TCP sur le port `22` vers le service GitLab Shell.

## NGINX Ingress {#nginx-ingress}

> [!warning]
> NGINX Ingress est déprécié depuis GitLab chart 19.0 et sera supprimé dans GitLab 20.0.
> Utilisez [Gateway API avec Envoy Gateway](#gateway-api-with-envoy-gateway) pour les nouveaux déploiements.

L'opérateur GitLab prend en charge le contrôleur NGINX Ingress sur Kubernetes et OpenShift. Lors de l'utilisation de NGINX Ingress, le port `22` doit être exposé sur le service NGINX pour activer Git via SSH.

GitLab maintient un [chart `NGINX-ingress` dupliqué](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork) qui peut être utilisé pour déployer des ressources NGINX configurées pour prendre en charge Git via SSH. Ce chart est déprécié et non pris en charge, mais reste disponible pour les déploiements existants.

NGINX Ingress est contrôlé par `nginx-ingress.enabled={true,false}` dans la ressource personnalisée GitLab. Lorsqu'il est défini sur `false`, vous pouvez utiliser une [instance NGINX externe](https://docs.gitlab.com/charts/advanced/external-nginx/).

## Routes OpenShift {#openshift-routes}

Les [Routes](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html) OpenShift constituent une solution Ingress intégrée aux clusters OpenShift. Lorsque vous désactivez NGINX Ingress en définissant `nginx-ingress.enabled=false` dans la ressource personnalisée GitLab, OpenShift convertit automatiquement les objets Ingress créés par l'opérateur en objets Route équivalents.

Git via SSH n'est pas pris en charge avec les Routes OpenShift, car celles-ci ne permettent pas d'exposer le trafic TCP (port `22`). Si vous avez besoin de Git via SSH sur OpenShift, vous pouvez :

- Utiliser Gateway API avec Envoy Gateway. Définissez `global.gatewayApi.enabled=true` dans votre ressource personnalisée GitLab.
- Utiliser le contrôleur NGINX Ingress. Définissez `nginx-ingress.enabled=true` (valeur par défaut).

Pour plus d'informations sur les options Ingress dans OpenShift, consultez la page [Ingress dans OpenShift](openshift_ingress.md).

## Éléments à prendre en compte {#considerations}

Vous trouverez ci-dessous des éléments à prendre en compte lors de l'utilisation d'Ingress.

### Utilisation d'un fournisseur Ingress tiers dans OpenShift {#using-a-third-party-ingress-provider-in-openshift}

Lors de l'utilisation d'un contrôleur Ingress tiers sur OpenShift, le contrôleur Ingress OpenShift peut entrer en conflit avec le contrôleur Ingress tiers dans certains cas.

Un exemple : le contrôleur NGINX Ingress définira une `ADDRESS` Ingress sur l'adresse IP externe du service NGINX, puis le contrôleur Ingress OpenShift la remplacera par le domaine de base du cluster. Cela peut entrer en conflit avec la configuration DNS, en particulier lors de l'utilisation d'un service comme [external-dns](https://github.com/kubernetes-sigs/external-dns) qui repose sur le fait que l'Ingress dispose d'une adresse IP pour pouvoir créer des enregistrements A afin de mapper l'URL vers ce service NGINX spécifique. C'est le cas dans l'environnement CI de l'opérateur GitLab.

Pour contourner ce problème, nous [corrigeons le contrôleur Ingress OpenShift](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26) pour qu'il gère uniquement les espaces de nommage spécifiques à OpenShift afin de garantir que les Ingresses que nous créons dans les espaces de nommage spécifiques à GitLab ne sont pas modifiés de manière indésirable.
