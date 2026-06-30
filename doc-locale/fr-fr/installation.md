---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Installation
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

> [!note]
> GitLab Operator présente des [limitations connues](_index.md#known-issues) et n'est adapté qu'à des scénarios spécifiques en production.

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->

GitLab Operator nécessite des instances externes de :

- [PostgreSQL](https://docs.gitlab.com/charts/advanced/external-db/)
- [Redis](https://docs.gitlab.com/charts/advanced/external-redis/)
- [Stockage d'objets](https://docs.gitlab.com/charts/advanced/external-object-storage/)

Pour les déploiements de production, suivez les
[architectures de référence Cloud Native](https://docs.gitlab.com/administration/reference_architectures).

Ce document décrit comment déployer GitLab Operator à l'aide de manifestes dans votre cluster Kubernetes ou OpenShift.

<!--This warning block is duplicated in `../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml`.
Changes should be reflected in both locations.-->

Si vous utilisez OpenShift, l'installation est généralement prise en charge par l'Operator Lifecycle Manager (OLM).
**L'installation via OLM est considérée comme expérimentale**. GitLab ne prend pas en charge les problèmes liés aux instances déployées via OLM.
Pour plus d'informations sur les problèmes potentiels liés à OLM, consultez le [ticket 241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241).

## Prérequis {#prerequisites}

1. [Créer ou utiliser un cluster Kubernetes ou OpenShift existant](#cluster)
1. Installer les services et logiciels prérequis
   - [Contrôleur Ingress/Gateway API](#external-traffic-routing)
   - [cert-manager](#tls-certificates)
   - [Serveur de métriques](#metrics)
1. [Configurer les services de noms de domaine](#configure-domain-name-services)

### Cluster {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

Pour créer un cluster Kubernetes traditionnel, envisagez d'utiliser les [outils officiels](https://kubernetes.io/docs/tasks/tools/) ou votre méthode d'installation préférée.

GitLab Operator prend en charge les versions Kubernetes suivantes :

| Version Kubernetes | Statut       | Version minimale de l'Operator |
|--------------------|--------------|-------------------------------|
| 1.35               | Prise en charge | 2.9.0                      |
| 1.34               | Prise en charge | 2.5.0                      |
| 1.33               | Prise en charge | 2.1.0                      |
| 1.32               | Obsolète     | 2.0.0                         |
| 1.31               | Non prise en charge | 1.9.0                  |

{{< /tab >}}

{{< tab title="OpenShift" >}}

GitLab Operator prend en charge les versions OpenShift suivantes :

| Version OpenShift | Statut       | Version minimale de l'Operator |
|-------------------|--------------|-------------------------------|
| 4.21              | Prise en charge | 2.9.0                      |
| 4.20              | Prise en charge | 2.6.0                      |
| 4.19              | Prise en charge | 2.2.0                      |
| 4.18              | Prise en charge | 1.9.0                      |
| 4.17              | Non prise en charge | 1.6.0                  |

{{< /tab >}}

{{< /tabs >}}

Nous visons la compatibilité avec les trois versions mineures les plus récentes de Kubernetes et les quatre versions mineures les plus récentes d'OpenShift simultanément. Lorsque la prise en charge d'une nouvelle version est ajoutée, les tests de la version prise en charge la plus ancienne sont abandonnés. Notre objectif est de fournir la prise en charge de l'Operator pour les nouvelles versions mineures de Kubernetes et d'OpenShift dans les trois mois suivant leur disponibilité initiale.

Pour plus de détails, [consultez notre politique de prise en charge de Kubernetes](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/).

> [!note]
> Pour certains composants, comme l'[agent pour Kubernetes](https://docs.gitlab.com/user/clusters/agent/)
> et le [chart GitLab](https://docs.gitlab.com/charts/installation/cloud/), GitLab peut prendre en charge différentes versions de cluster.

Nous accueillons tout problème de compatibilité avec des versions plus récentes que celles listées ci-dessus dans notre [système de suivi des tickets](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues).

Certaines fonctionnalités de GitLab peuvent ne pas fonctionner sur les versions obsolètes et les versions antérieures à celles listées ci-dessus.

L'Operator prend en charge les architectures x86-64 et ARM64. Bien que les builds ARM64 soient disponibles depuis la version 16.7, la prise en charge complète et la couverture des tests sont disponibles à partir de la version 18.8.

### Routage du trafic externe {#external-traffic-routing}

Le routage du trafic externe est nécessaire pour permettre l'accès à l'application.
[Gateway API](https://gateway-api.sigs.k8s.io/) avec [Envoy Gateway](https://gateway.envoyproxy.io/)
est l'approche recommandée pour les nouveaux déploiements. Pour les détails de configuration et les fournisseurs alternatifs, consultez la [documentation Gateway API](gatewayapi.md).

Le contrôleur NGINX Ingress intégré est obsolète depuis le chart GitLab 19.0 et sera supprimé dans GitLab 20.0. Un [contrôleur NGINX Ingress externe](https://docs.gitlab.com/charts/advanced/external-ingress/) peut être utilisé comme alternative.

### Certificats TLS {#tls-certificates}

Pour créer un certificat pour le webhook Kubernetes de l'Operator, [cert-manager](https://cert-manager.io) est utilisé. Vous devriez également utiliser [cert-manager](https://cert-manager.io) pour les certificats GitLab.

L'Operator ayant besoin d'un certificat pour le webhook Kubernetes, vous ne pouvez pas utiliser le cert-manager intégré au chart GitLab. Installez donc cert-manager avant d'installer l'Operator.

Suivez la [documentation d'installation](https://cert-manager.io/docs/installation/) pour installer une [version de cert-manager prise en charge](https://cert-manager.io/docs/releases/) adaptée à votre plateforme et à vos outils.

### Métriques {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

Installez le [serveur de métriques](https://github.com/kubernetes-sigs/metrics-server#installation) afin que les HorizontalPodAutoscalers puissent récupérer les métriques des pods.

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShift est livré avec [Prometheus Adapter](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html) par défaut. Il vous suffit donc de définir `spec.chart.values.prometheus.install=false` dans votre ressource personnalisée GitLab pour empêcher GitLab Operator d'installer une autre instance.

{{< /tab >}}

{{< /tabs >}}

### Configurer les services de noms de domaine {#configure-domain-name-services}

Vous avez besoin d'un domaine accessible depuis Internet auquel vous pouvez ajouter un enregistrement DNS.

Consultez notre [documentation sur la mise en réseau et le DNS](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns) pour plus de détails sur la connexion de votre domaine aux composants GitLab. Vous utiliserez la configuration mentionnée dans cette section lors de la définition de votre ressource personnalisée GitLab (CR).

L'Ingress dans OpenShift nécessite une attention particulière. Consultez nos [notes sur l'Ingress OpenShift](openshift_ingress.md) pour plus d'informations.

## Installation de GitLab Operator {#installing-the-gitlab-operator}

Commencez par sélectionner une méthode d'installation.

{{< tabs >}}

{{< tab title="Manifest" >}}

Récupérez d'abord un manifeste de version depuis la
[page des versions de l'Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases).
Chaque version publie quatre manifestes ; choisissez celui qui correspond à votre plateforme cible et à la portée RBAC dont vous avez besoin :

| Manifeste                                      | Plateforme | Portée RBAC                                                                                                    |
|------------------------------------------------|------------|----------------------------------------------------------------------------------------------------------------|
| `gitlab-operator-kubernetes.yaml`              | Kubernetes | À l'échelle du cluster : `ClusterRole`/`ClusterRoleBinding` ; l'Operator surveille les CR GitLab dans tous les espaces de noms. |
| `gitlab-operator-kubernetes-namespaced.yaml`   | Kubernetes | Limité à l'espace de noms : `Role`/`RoleBinding` limité à l'espace de noms d'installation ; l'Operator surveille uniquement cet espace de noms. Un petit `ClusterRole` est toujours requis pour les ressources à portée cluster gérées par l'Operator. |
| `gitlab-operator-openshift.yaml`               | OpenShift  | À l'échelle du cluster (comme ci-dessus).                                                                      |
| `gitlab-operator-openshift-namespaced.yaml`    | OpenShift  | Limité à l'espace de noms (comme ci-dessus).                                                                   |

Utilisez la variante à l'échelle du cluster lorsqu'un seul Operator doit gérer des instances GitLab dans plusieurs espaces de noms. Utilisez la variante limitée à l'espace de noms lorsque l'Operator doit être confiné à un seul espace de noms, par exemple parce que l'attribution de droits RBAC à l'échelle du cluster n'est pas autorisée dans le cluster cible. Avec la variante limitée à l'espace de noms, la ressource personnalisée GitLab doit être créée dans le même espace de noms que l'Operator.

Créez ensuite l'espace de noms dans lequel l'Operator sera installé.
Dans le manifeste, l'espace de noms est défini sur `gitlab-system` par défaut.
Pour modifier l'espace de noms, mettez à jour le manifeste manuellement ou envisagez d'utiliser le chart Helm, où cette clé et d'autres peuvent être facilement configurées.

```shell
kubectl create namespace gitlab-system
```

Enfin, appliquez le manifeste :

```shell
kubectl apply -f gitlab-operator-<platform>.yaml
```

{{< /tab >}}

{{< tab title="Helm Chart" >}}

Ajoutez d'abord le dépôt Helm GitLab et récupérez les dernières mises à jour.

```shell
helm repo add gitlab https://charts.gitlab.io
helm repo update
```

Vous pouvez ensuite installer le chart GitLab Operator :

```shell
helm install gitlab-operator gitlab/gitlab-operator \
  --create-namespace \
  --namespace gitlab-system
```

Consultez [`values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml)
pour toutes les options de configuration disponibles.

{{< /tab >}}

{{< tab title="OLM" >}}

GitLab Operator est disponible dans les canaux OLM suivants :

- [OperatorHub.io](https://operatorhub.io/operator/gitlab-operator-kubernetes)
- [OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod), dans l'OperatorHub intégré à OpenShift et OKD
- [Red Hat Ecosystem Catalog](https://catalog.redhat.com/en/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f)

{{< /tab >}}

{{< /tabs >}}

Confirmez l'installation en vérifiant le statut du déploiement de l'Operator :

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

### NGINX Ingress et Prometheus intégrés en mode limité à l'espace de noms {#bundled-nginx-ingress-and-prometheus-in-namespaced-mode}

Les variantes de manifeste limitées à l'espace de noms (et les installations Helm avec
`watchCluster=false`) omettent le `ClusterRole` et le `ClusterRoleBinding` pour
le contrôleur NGINX Ingress intégré au chart et le serveur Prometheus. Ces
deux composants sont conçus pour fonctionner à l'échelle du cluster :

- NGINX Ingress surveille les `Ingresses` dans tous les espaces de noms et lit
  les ressources à portée cluster telles que `nodes` et `ingressclasses`.
- Le serveur Prometheus découvre et collecte les cibles à travers le cluster
  et nécessite l'accès à `nodes`, `nodes/proxy`, `nodes/metrics` et à l'URL
  non-ressource `/metrics`.

Si vous installez l'Operator en mode limité à l'espace de noms et souhaitez utiliser ces
composants intégrés, vous devez fournir vous-même les autorisations à portée cluster
(généralement en liant les `ServiceAccount` `gitlab-nginx-ingress` /
`gitlab-prometheus-server` du chart à un `ClusterRole` géré en externe,
et pour NGINX Ingress en passant également `--watch-namespace` au contrôleur).

L'approche recommandée en mode limité à l'espace de noms est d'utiliser des solutions
de surveillance et d'Ingress/Gateway API gérées en externe.

## Installation de GitLab {#installing-gitlab}

1. Créez une ressource personnalisée GitLab (CR).

   Créez un nouveau fichier nommé par exemple `mygitlab.yaml`.

   Voici un exemple du contenu à placer dans ce fichier :

   ```yaml
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   spec:
     chart:
       version: "X.Y.Z" # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/<OPERATOR_VERSION>/CHART_VERSIONS
       values:
         global:
           hosts:
             domain: example.com # use a real domain here
           ingress:
             configureCertmanager: true
         certmanager-issuer:
           email: youremail@example.com # use your real email address here
   ```

   Pour plus de détails sur les options de configuration à utiliser sous `spec.chart.values`,
   consultez la [documentation du chart Helm GitLab](https://docs.gitlab.com/charts/charts/).

1. Déployez une instance GitLab à l'aide de votre nouvelle CR GitLab.

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   Cette commande envoie votre CR GitLab au cluster pour que GitLab Operator la réconcilie. Vous pouvez suivre la progression en consultant les journaux du pod du contrôleur :

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   Vous pouvez également lister les ressources GitLab et vérifier leur statut :

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

   Une fois la CR réconciliée (le statut de la ressource GitLab est `Running`), vous pouvez accéder à GitLab dans votre navigateur à l'adresse `https://gitlab.example.com`.

Pour vous connecter, vous devez récupérer le mot de passe root initial de votre déploiement. Consultez la [documentation du chart Helm](https://docs.gitlab.com/charts/installation/deployment/#initial-login) pour obtenir des instructions supplémentaires.

## Étapes suivantes recommandées {#recommended-next-steps}

Une fois l'installation terminée, envisagez de suivre les
[étapes suivantes recommandées](https://docs.gitlab.com/install/next_steps/),
notamment les options d'authentification et les restrictions d'inscription.

### OpenShift {#openshift}

Si vous utilisez OpenShift, modifiez la stratégie d'approbation de GitLab Operator en passant du mode automatique (par défaut) au mode manuel. Cela empêche OpenShift d'installer de nouvelles versions de l'Operator jusqu'à ce qu'une [approbation soit donnée](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators).

Vous pouvez également définir un [`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster) personnalisé pour épingler la version de l'Operator ou effectuer une mise à niveau vers une version non récente.

- La stratégie d'approbation peut être modifiée depuis la [console web OpenShift](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf)
  ou en [modifiant l'abonnement](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm).
- Définissez `.spec.approved` sur `true` dans l'`InstallPlan` pour approuver une mise à niveau manuelle.
- Chaque GitLab Operator prend en charge un sous-ensemble défini de versions du chart GitLab : les mises à niveau de GitLab Operator doivent également inclure la mise à jour de la version du chart dans la ressource personnalisée GitLab.
- Si GitLab Operator et la version du chart Helm GitLab spécifiée sont incompatibles, les modifications de configuration du chart peuvent échouer avec des [erreurs concernant la version du chart Helm GitLab](gitlab_upgrades.md).

> [!note]
> [OLM ne prend pas en charge le retour à une version antérieure des Operators](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177).

## Désinstaller GitLab Operator {#uninstall-the-gitlab-operator}

Suivez les étapes ci-dessous pour supprimer GitLab Operator et les ressources associées.

Points à noter avant de désinstaller l'Operator :

- L'Operator ne supprime pas les Persistent Volume Claims ni les Secrets lors de la suppression d'une instance GitLab.
- Lors de la suppression de l'Operator, l'espace de noms dans lequel il est installé (`gitlab-system` par défaut) n'est pas supprimé automatiquement. Cela garantit que les volumes persistants ne sont pas perdus involontairement.

### Désinstaller une instance de GitLab {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

Cette commande supprime l'instance GitLab et tous les objets associés, à l'exception des Persistent Volume Claims (comme indiqué ci-dessus).

### Désinstaller GitLab Operator {#uninstall-the-gitlab-operator}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

Cette commande supprime les ressources de l'Operator, y compris le déploiement en cours de l'Operator. Elle ne supprime **pas** les objets associés à une instance GitLab.

## Résoudre les problèmes de GitLab Operator {#troubleshoot-the-gitlab-operator}

Pour obtenir des informations sur la résolution des problèmes de GitLab Operator, consultez la page [Résolution des problèmes](troubleshooting.md).
