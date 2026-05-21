---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Installation
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

> [!note] L'opérateur GitLab présente des [limitations connues](_index.md#known-issues) et n'est adapté qu'à des scénarios spécifiques en production.

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->

> [!warning] 
> Les valeurs par défaut de la ressource personnalisée GitLab **ne sont pas destinées à une utilisation en production**. Avec ces valeurs, l'opérateur GitLab crée une instance GitLab dans laquelle tous les services, y compris les données persistantes, sont déployés dans un cluster Kubernetes, ce qui **ne convient pas aux charges de travail en production**. Pour les déploiements en production, vous **devez** suivre les [architectures de référence cloud-native hybrides](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid). GitLab ne prendra en charge aucun problème lié à PostgreSQL, Redis, Gitaly, Praefect ou MinIO déployés à l'intérieur d'un cluster Kubernetes.

Ce document décrit comment déployer l'opérateur GitLab en utilisant des manifestes dans votre cluster Kubernetes ou OpenShift.

<!--This warning block is duplicated in `../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml`.
Changes should be reflected in both locations.-->

Si vous utilisez OpenShift, l'installation est généralement gérée par Operator Lifecycle Manager (OLM). **L'installation via OLM est considérée comme expérimentale**. GitLab ne prend en charge aucun problème lié aux instances déployées via OLM. Pour plus d'informations sur les problèmes potentiels avec OLM, consultez le [ticket 241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241).

## Prérequis {#prerequisites}

1. [Créez ou utilisez un cluster Kubernetes ou OpenShift existant](#cluster)
1. Installez les services et logiciels prérequis
   - [Contrôleur Ingress](#ingress-controller)
   - [cert-manager](#tls-certificates)
   - [Serveur de métriques](#metrics)
1. [Configurez les services DNS](#configure-domain-name-services)

### Cluster {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

Pour créer un cluster Kubernetes traditionnel, utilisez les [outils officiels](https://kubernetes.io/docs/tasks/tools/) ou votre méthode d'installation préférée.

L'opérateur GitLab prend en charge les versions Kubernetes suivantes :

| Version Kubernetes | Statut      | Version minimale de l'opérateur GitLab |
|--------------------|-------------|--------------------------|
| 1.35               | Prise en charge   | 2.9.0                    |
| 1.34               | Prise en charge   | 2.5.0                    |
| 1.33               | Prise en charge   | 2.1.0                    |
| 1.32               | Obsolète  | 2.0.0                    |
| 1.31               | Non prise en charge | 1.9.0                    |

{{< /tab >}}

{{< tab title="OpenShift" >}}

L'opérateur GitLab prend en charge les versions OpenShift suivantes :

| Version OpenShift | Statut      | Version minimale de l'opérateur GitLab |
|-------------------|-------------|--------------------------|
| 4.21              | Prise en charge   | 2.9.0                    |
| 4.20              | Prise en charge   | 2.6.0                    |
| 4.19              | Prise en charge   | 2.2.0                    |
| 4.18              | Prise en charge   | 1.9.0                    |
| 4.17              | Non prise en charge | 1.6.0                    |

{{< /tab >}}

{{< /tabs >}}

Nous visons la compatibilité avec les trois versions mineures les plus récentes de Kubernetes et les quatre releases mineures les plus récentes d'OpenShift simultanément. Lorsque la prise en charge d'une nouvelle version est ajoutée, les tests pour la version prise en charge la plus ancienne sont abandonnés. Notre objectif est de fournir une prise en charge de l'opérateur GitLab pour les nouvelles releases mineures de Kubernetes et OpenShift dans les trois mois suivant leur disponibilité initiale.

Pour plus de détails, [consultez notre politique de prise en charge Kubernetes](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/).

> [!note] 
> Pour certains composants, comme l'[agent pour Kubernetes](https://docs.gitlab.com/user/clusters/agent/) et le [chart GitLab](https://docs.gitlab.com/charts/installation/cloud/), GitLab peut prendre en charge différentes versions de cluster.

N'hésitez pas à signaler tout problème de compatibilité avec des releases plus récentes que celles indiquées ci-dessus dans notre [outil de suivi des tickets](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues).

Certaines fonctionnalités de GitLab peuvent ne pas fonctionner sur les versions obsolètes et les versions antérieures aux versions listées ci-dessus.

L'opérateur GitLab prend en charge x86-64 et ARM64. Bien que les builds ARM64 soient disponibles depuis la version 16.7, la prise en charge complète et la couverture de tests sont disponibles à partir de la version 18.8.

### Contrôleur Ingress {#ingress-controller}

Un contrôleur Ingress est nécessaire pour fournir un accès externe à l'application et sécuriser la communication entre les composants.

L'opérateur GitLab déploie par défaut notre [chart NGINX dupliqué depuis le GitLab Helm Chart](https://docs.gitlab.com/charts/charts/nginx/).

Si vous préférez utiliser un contrôleur Ingress externe, utilisez [NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/) de la communauté Kubernetes pour déployer un contrôleur Ingress. Suivez les instructions pertinentes dans le lien en fonction de votre plateforme et des outils préférés. Notez la valeur de la classe Ingress pour plus tard (elle est généralement définie par défaut sur `nginx`). Lors de la configuration de la ressource personnalisée GitLab, veillez à définir `nginx-ingress.enabled=false` pour désactiver les objets NGINX du GitLab Helm Chart.

### Certificats TLS {#tls-certificates}

Pour créer un certificat pour le webhook Kubernetes de l'opérateur GitLab, [cert-manager](https://cert-manager.io) est utilisé. Vous devriez également utiliser [cert-manager](https://cert-manager.io) pour les certificats GitLab.

Comme l'opérateur GitLab a besoin d'un certificat pour le webhook Kubernetes, vous ne pouvez pas utiliser le cert-manager fourni avec le chart GitLab. À la place, installez cert-manager avant d'installer l'opérateur GitLab.

Suivez la [documentation d'installation](https://cert-manager.io/docs/installation/) pour installer une [version de cert-manager prise en charge](https://cert-manager.io/docs/releases/) pour votre plateforme et vos outils.

### Métriques {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

Installez le [serveur de métriques](https://github.com/kubernetes-sigs/metrics-server#installation) afin que les HorizontalPodAutoscalers puissent récupérer les métriques des pods.

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShift est fourni avec [Prometheus Adapter](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html) par défaut, il vous suffit donc de définir `spec.chart.values.prometheus.install=false` dans votre ressource personnalisée GitLab pour empêcher l'opérateur GitLab d'installer une autre instance.

{{< /tab >}}

{{< /tabs >}}

### Configurer les services DNS {#configure-domain-name-services}

Vous avez besoin d'un domaine accessible depuis Internet auquel vous pouvez ajouter un enregistrement DNS.

Consultez notre [documentation sur la mise en réseau et le système de nom de domaine (DNS)](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns) pour plus de détails sur la connexion de votre domaine aux composants GitLab. Vous utilisez la configuration mentionnée dans cette section lors de la définition de votre ressource personnalisée GitLab.

Ingress dans OpenShift nécessite une attention particulière. Consultez nos [notes sur OpenShift Ingress](openshift_ingress.md) pour plus d'informations.

## Installation de l'opérateur GitLab {#installing-the-gitlab-operator}

Commencez par sélectionner une méthode d'installation.

{{< tabs >}}

{{< tab title="Manifest" >}}

Tout d'abord, récupérez un manifeste de release depuis la [page des releases de l'opérateur GitLab](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases). Sélectionnez le manifeste correspondant à votre plateforme cible : Kubernetes ou OpenShift.

Ensuite, créez l'espace de nommage dans lequel l'opérateur GitLab sera installé. Dans le manifeste, l'espace de nommage est défini par défaut sur `gitlab-system`. Pour modifier l'espace de nommage, mettez à jour le manifeste manuellement ou utilisez le chart Helm où cette clé et d'autres peuvent être facilement configurées.

```shell
kubectl create namespace gitlab-system
```

Enfin, appliquez le manifeste :

```shell
kubectl apply -f gitlab-operator-<platform>.yaml
```

{{< /tab >}}

{{< tab title="Helm Chart" >}}

Tout d'abord, ajoutez le dépôt Helm GitLab et récupérez les dernières mises à jour.

```shell
helm repo add gitlab https://charts.gitlab.io
helm repo update
```

Vous pouvez ensuite installer le chart de l'opérateur GitLab :

```shell
helm install gitlab-operator gitlab/gitlab-operator \
  --create-namespace \
  --namespace gitlab-system
```

Consultez [`values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml) pour toutes les options de configuration disponibles.

{{< /tab >}}

{{< tab title="OLM" >}}

L'opérateur GitLab est disponible dans les canaux OLM suivants :

- [OperatorHub.io](https://operatorhub.io/operator/gitlab-operator-kubernetes)
- [OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod), dans l'OperatorHub intégré à OpenShift et OKD
- [Ecosystem Catalog de Red Hat](https://catalog.redhat.com/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f)

{{< /tab >}}

{{< /tabs >}}

Confirmez l'installation en vérifiant le statut du déploiement de l'opérateur GitLab :

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

## Installation de GitLab {#installing-gitlab}

1. Créez une ressource personnalisée GitLab.

   Créez un nouveau fichier nommé par exemple `mygitlab.yaml`.

   Voici un exemple du contenu à mettre dans ce fichier :

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

   Pour plus de détails sur les options de configuration à utiliser sous `spec.chart.values`, consultez la [documentation du GitLab Helm Chart](https://docs.gitlab.com/charts/charts/).

1. Déployez une instance GitLab en utilisant votre ressource personnalisée GitLab.

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   Cette commande envoie votre ressource personnalisée GitLab au cluster pour que l'opérateur GitLab la réconcilie. Vous pouvez suivre la progression en affichant les logs du pod du contrôleur :

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   Vous pouvez également afficher les ressources GitLab et vérifier leur statut :

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

   Lorsque la ressource personnalisée est réconciliée (le statut de la ressource GitLab est `Running`), vous pouvez accéder à GitLab dans votre navigateur à l'adresse `https://gitlab.example.com`.

Pour vous connecter, vous devez récupérer le mot de passe root initial de votre déploiement. Consultez la [documentation du Helm Chart](https://docs.gitlab.com/charts/installation/deployment/#initial-login) pour des instructions supplémentaires.

## Prochaines étapes recommandées {#recommended-next-steps}

Après avoir terminé votre installation, pensez à suivre les [prochaines étapes recommandées](https://docs.gitlab.com/install/next_steps/), notamment les options d'authentification et les restrictions d'inscription.

### OpenShift {#openshift}

Si vous utilisez OpenShift, changez la stratégie d'approbation pour l'opérateur GitLab de automatique (valeur par défaut) à manuelle. Cela empêche OpenShift d'installer de nouvelles versions de l'opérateur GitLab jusqu'à ce qu'une [approbation soit donnée](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators).

Vous pouvez également définir un [`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster) personnalisé pour épingler la version de l'opérateur GitLab ou pour effectuer une mise à niveau vers une version qui n'est pas la dernière version.

- La stratégie d'approbation peut être modifiée depuis la [console Web OpenShift](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf) ou en [modifiant l'abonnement](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm).
- Définissez `.spec.approved` sur `true` de l'`InstallPlan` pour approuver une mise à niveau manuelle.
- Chaque opérateur GitLab prend en charge un sous-ensemble défini de versions du chart GitLab : les mises à niveau de l'opérateur GitLab doivent également inclure la mise à jour de la version du chart dans la ressource personnalisée GitLab.
- Si l'opérateur GitLab et la version spécifiée du chart Helm GitLab sont incompatibles, les modifications de configuration du chart peuvent échouer avec des [erreurs concernant la version du chart Helm GitLab](gitlab_upgrades.md).

> [!note] 
> [OLM ne prend pas en charge le retour à la version précédente des Operators](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177).

## Désinstaller l'opérateur GitLab {#uninstall-the-gitlab-operator}

Suivez les étapes ci-dessous pour supprimer l'opérateur GitLab et ses ressources associées.

Points à noter avant de désinstaller l'opérateur GitLab :

- L'opérateur GitLab ne supprime pas les demandes de volume persistant ni les secrets lorsqu'une instance GitLab est supprimée.
- Lors de la suppression de l'opérateur GitLab, l'espace de nommage dans lequel il est installé (`gitlab-system` par défaut) n'est pas supprimé automatiquement. Cela garantit que les volumes persistants ne sont pas perdus involontairement.

### Désinstaller une instance de GitLab {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

Cela supprime l'instance GitLab et tous les objets associés, à l'exception des demandes de volume persistant comme indiqué ci-dessus.

### Désinstaller l'opérateur GitLab {#uninstall-the-gitlab-operator-1}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

Cela supprime les ressources de l'opérateur GitLab, y compris le déploiement en cours d'exécution de l'opérateur GitLab. Cela ne **supprime pas** les objets associés à une instance GitLab.

## Dépanner l'opérateur GitLab {#troubleshoot-the-gitlab-operator}

Pour des informations sur le dépannage de l'opérateur GitLab, consultez la page [dépannage](troubleshooting.md).
