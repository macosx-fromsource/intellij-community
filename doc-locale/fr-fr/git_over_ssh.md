---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Prise en charge de Git via SSH
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

Ce document fournit des recommandations de configuration pour Git via SSH dans différents environnements et plateformes.

## Vue d'ensemble {#overview}

Le [chart Helm GitLab Shell](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/) fournit un serveur SSH configuré pour l'accès Git SSH à GitLab. Ce composant doit être exposé en dehors du cluster sur le port `22`.

L'opérateur GitLab déploie `gitlab-shell` lorsque `gitlab.gitlab-shell.enabled` est défini sur `true`, ce qui correspond au paramètre par défaut.

Voici un récapitulatif des exigences selon la plateforme cible :

| Avez-vous besoin de Git via SSH ? | Kubernetes                                                                                                    | OpenShift |
|-----------------------------------|---------------------------------------------------------------------------------------------------------------|-----------|
| Non                               | Vous devez utiliser l'un des fournisseurs NGINX Ingress ci-dessous (Kubernetes ne dispose pas de fournisseur Ingress intégré). | Vous n'avez pas besoin des fournisseurs Ingress ci-dessous — vous pouvez utiliser les Routes intégrées comme fournisseur Ingress. |
| Oui                               | Vous devez utiliser l'un des fournisseurs NGINX Ingress ci-dessous (Kubernetes ne dispose pas de fournisseur Ingress intégré). | Vous devez utiliser l'un des fournisseurs Ingress ci-dessous — les Routes ne permettent pas d'exposer le port `22`. |

## Fournisseurs Ingress {#ingress-providers}

Vous trouverez ci-dessous la liste des fournisseurs Ingress, accompagnée de remarques pertinentes et de détails propres à chaque plateforme.

### Chart Helm NGINX-Ingress {#nginx-ingress-helm-chart}

GitLab maintient un [fork du chart `NGINX-ingress`](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork) qui permet de déployer des ressources NGINX modifiées pour prendre en charge Git via SSH « prêt à l'emploi ».

Il s'agit de la configuration par défaut lors de l'utilisation de l'opérateur GitLab, contrôlée par `nginx-ingress.enabled={true,false}` dans le CR GitLab. Lorsque ce paramètre est défini sur `false`, vous pouvez utiliser une [instance NGINX externe](https://docs.gitlab.com/charts/advanced/external-nginx/).

Ce fournisseur Ingress peut être utilisé aussi bien sur Kubernetes que sur OpenShift.

Des informations complémentaires sur les options d'installation du fournisseur NGINX Ingress sont disponibles dans notre [documentation d'installation](installation.md#ingress-controller).

### Opérateur NGINX Ingress {#nginx-ingress-operator}

En alternative au fork intégré du chart Helm NGINX-Ingress, l'[opérateur NGINX Ingress](https://github.com/nginxinc/nginx-ingress-operator) peut être utilisé pour exposer `gitlab-shell`.

Cette option présente certaines limitations :

- Les définitions de ressources personnalisées TransportServer/GlobalConfiguration de NGINX Inc. sont considérées comme une fonctionnalité en préversion, et leur utilisation en production est déconseillée.
- L'opérateur NGINX Inc. est encore relativement récent, à la version 0.3.0 seulement. Il offre bien moins d'options de configuration que les charts Helm plus matures, quelle que soit leur variante.
- Cette option nécessite toujours d'exposer manuellement le port `22` sur le service NGINX (ce paramètre n'est pas configurable dans le CR NGINXIngressController).

Des recherches plus approfondies sont consignées dans [#58](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/58#note_585883916).

### Routes OpenShift {#openshift-routes}

Les [Routes](https://docs.openshift.com/container-platform/3.4/architecture/core_concepts/routes.html) OpenShift sont un composant intégré aux clusters OpenShift. Elles constituent l'équivalent OpenShift des [Ingresses Kubernetes](https://kubernetes.io/docs/concepts/services-networking/ingress/).

Lors d'un déploiement sur OpenShift, vous pouvez définir `nginx-ingress.enabled=false` dans le CR GitLab et laisser les Routes OpenShift gérer le flux du trafic externe. Lorsque l'opérateur GitLab réconcilie les objets Ingress, OpenShift crée automatiquement un objet Route équivalent, mappé sur le domaine de base du cluster.

Notez que les Routes OpenShift ne permettent pas d'exposer le trafic TCP (SSH sur le port `22`) et ne peuvent donc pas être utilisées pour Git via SSH avec `gitlab-shell`.

## Considérations {#considerations}

Vous trouverez ci-dessous les points à prendre en compte lors de l'utilisation d'Ingress.

### Utilisation d'un fournisseur Ingress tiers sur OpenShift {#using-a-third-party-ingress-provider-in-openshift}

Lors de l'utilisation d'un contrôleur Ingress tiers sur OpenShift, le contrôleur Ingress OpenShift peut entrer en conflit avec le contrôleur Ingress tiers dans certains cas.

Par exemple, le contrôleur NGINX Ingress définit une `ADDRESS` Ingress sur l'adresse IP externe du service NGINX, puis le contrôleur Ingress OpenShift la remplace par le domaine de base du cluster. Cela peut créer des conflits avec la configuration DNS, notamment lors de l'utilisation d'un service tel qu'[external-dns](https://github.com/kubernetes-sigs/external-dns), qui repose sur la présence d'une adresse IP dans l'Ingress pour créer des enregistrements A permettant de mapper l'URL vers ce service NGINX spécifique. C'est le cas dans l'environnement CI de l'opérateur GitLab.

Pour contourner ce problème, nous [appliquons un patch au contrôleur Ingress OpenShift](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26) afin qu'il ne gère que les espaces de noms propres à OpenShift, garantissant ainsi que les Ingresses créées dans les espaces de noms GitLab ne sont pas modifiées de manière indésirable.
