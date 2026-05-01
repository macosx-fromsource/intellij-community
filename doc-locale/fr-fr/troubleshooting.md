---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Dépannage de l'Operator
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

Ce document regroupe des notes et conseils pour vous aider à résoudre les problèmes liés à l'installation de l'Operator GitLab et au déploiement d'une instance GitLab à partir de la ressource personnalisée GitLab.

## Problèmes d'installation {#installation-problems}

Le dépannage de l'installation de l'operator dans un environnement Kubernetes est similaire à celui de toute autre charge de travail Kubernetes. Après avoir déployé le manifeste de l'operator, surveillez la sortie de `kubectl describe` pour le Pod de l'operator ou `kubectl get events -n <namespace>`. Cela permettra d'identifier tout problème lié à la récupération de l'image de l'operator ou à toute autre condition préalable au démarrage de l'operator.

Si l'operator démarre mais se termine prématurément, l'examen des journaux de l'operator peut fournir des informations pour déterminer la cause de l'arrêt du Pod. Pour ce faire, exécutez la commande suivante :

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

Par ailleurs, l'operator dépend de Cert Manager pour créer un certificat TLS nécessaire à son bon fonctionnement. Ce certificat TLS est créé sous forme de Secret et monté en tant que volume sur le Pod de l'operator. Les problèmes liés à l'obtention du certificat TLS peuvent être consultés dans le journal des événements du Namespace.

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

L'étape suivante consiste à inspecter les journaux de Cert Manager afin d'identifier les problèmes à l'origine de l'échec de création du certificat TLS.

### Problèmes spécifiques à OpenShift {#openshift-specific-problems}

OpenShift applique un modèle de sécurité plus restrictif ; par conséquent, l'operator GitLab doit être installé avec le compte administrateur du cluster. Les comptes développeur ne disposent pas des privilèges nécessaires pour permettre à l'operator de fonctionner correctement.

Si les Pods du [contrôleur Ingress NGINX](https://github.com/kubernetes/ingress-nginx) ne peuvent pas être provisionnés en raison de paramètres SCC invalides, comme décrit dans [ce ticket](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762), la solution appropriée consiste à mettre à jour le SCC depuis le dépôt afin d'autoriser le démarrage d'Ingress NGINX dans votre cluster OpenShift :

1. Récupérez le [dernier manifeste OpenShift](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases) pour l'Operator GitLab. Vous avez besoin du fichier `gitlab-operator-openshift-VERSION.yaml`
1. Extrayez le SCC

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. Appliquez `scc.yaml` à votre cluster :

   ```shell
   kubectl apply -f scc.yaml
   ```

L'installation à partir d'un manifeste publié depuis la page Releases du dépôt de l'Operator GitLab ne présente pas ce problème, car le SCC y est inclus.
Tickets associés pour les objets non pris en charge dans les OperatorHubs :

- [Add support for IngressClass CR · Issue #5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [Add support for OpenShift's SCC · Issue #2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## Problèmes de déploiement de l'instance GitLab {#problems-with-deployment-of-gitlab-instance}

En complément des informations présentées ici, nous vous recommandons de consulter la [documentation de dépannage](https://docs.gitlab.com/charts/troubleshooting/) du chart Helm GitLab.

### Services principaux non prêts {#core-services-not-ready}

L'Operator GitLab repose sur l'installation d'instances Redis, PostgreSQL et Gitaly, désignées comme les services principaux. Si, après le déploiement d'une ressource client GitLab, un nombre excessif de messages de journaux de l'operator indique que les services principaux ne sont pas prêts, c'est l'un de ces services qui rencontre des difficultés à devenir opérationnel.

Vérifiez en particulier les endpoints de chacun de ces services pour vous assurer qu'ils sont bien connectés au Pod du service. Cela peut également indiquer que le cluster ne dispose pas de ressources suffisantes pour prendre en charge l'instance GitLab et que des nœuds supplémentaires doivent être ajoutés au cluster.

Le ticket [#305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305)
a été créé pour suivre le signalement du service principal qui bloque le déploiement de l'instance GitLab.

### Interface utilisateur GitLab inaccessible (les Ingresses n'ont pas d'adresse et/ou les Challenges CertManager échouent) {#gitlab-ui-unreachable-ingresses-have-no-address-and-or-certmanager-challenges-failing}

Le manifeste d'installation de l'Operator GitLab et le chart Helm utilisent `gitlab` comme préfixe pour tous les noms de ressources par défaut, sauf si `nameOverride` est spécifié dans les valeurs Helm.

Par conséquent, l'IngressClass NGINX sera nommée `gitlab-nginx`. Si un nom de release autre que `gitlab` est spécifié dans la ressource personnalisée GitLab sous `metadata.name`, le nom de l'IngressClass par défaut doit être défini explicitement sous `global.ingress.class` :

Par exemple : si `metadata.name` est défini sur `demo`, définissez `global.ingress.class=gitlab-nginx` :

```yaml
apiVersion: apps.gitlab.com/v1beta1
kind: GitLab
metadata:
  name: demo
spec:
  chart:
    version: "X.Y.Z"
    values:
      global:
        ingress:
          # Use the correct IngressClass name.
          class: gitlab-nginx
```

Sans ce paramètre explicite, les Ingresses tenteraient de trouver un Ingress nommé `demo-nginx`, qui n'existe pas.

### Pods du contrôleur Ingress NGINX manquants {#nginx-ingress-controller-pods-missing}

Dans un environnement OpenShift, le [contrôleur Ingress NGINX](https://kubernetes.github.io/ingress-nginx/) est utilisé à la place des Routes OpenShift pour diriger le trafic vers l'instance GitLab (HTTPS et SSH). Si vous rencontrez un problème de connexion à l'instance GitLab, vérifiez d'abord qu'un déploiement du contrôleur Ingress NGINX est bien présent.

Si un déploiement est présent, vérifiez la colonne `READY` dans la sortie de `kubectl get deploy`. Si le statut `READY` est `0/0`, inspectez la sortie de `kubectl get events -n <namespace> | grep -i nginx` en recherchant les messages indiquant que la contrainte de contexte de sécurité (SCC) a été violée.

Cela indique que les ressources RBAC NGINX pour OpenShift n'ont pas été déployées. Le manifeste de l'operator pour OpenShift doit être réappliqué avec la commande suivante :

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

Une fois le manifeste appliqué, il peut être nécessaire de supprimer le déploiement du contrôleur Ingress pour acquérir correctement le SCC et permettre au contrôleur Ingress de créer les Pods correctement.

### Les autoscalers horizontaux de pods ne se mettent pas à l'échelle {#horizontal-pod-autoscalers-are-not-scaling}

Si les autoscalers horizontaux de pods (HPA) ne font pas évoluer le nombre de pods en fonction de la charge de trafic, vérifiez qu'un Metrics Server est bien installé. Dans un cluster Kubernetes, le Metrics Server est un composant supplémentaire qui doit être installé séparément. La procédure d'installation est disponible dans la [documentation d'installation](installation.md#metrics).

Un cluster OpenShift dispose d'un Metrics Server intégré ; par conséquent, les HPA devraient fonctionner correctement.

### Restauration des données lors de modifications de la configuration PersistentVolumeClaim {#restoring-data-when-persistentvolumeclaim-configuration-changes}

Lorsque vous utilisez des composants tels que MinIO pour la persistance des données, il peut parfois être nécessaire de vous reconnecter à un PersistentVolume précédent.

Par exemple, la merge request [!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419) a remplacé les composants MinIO définis par l'Operator par les composants MinIO issus des charts Helm GitLab. Dans le cadre de cette modification, les noms des objets ont changé, y compris celui du PersistentVolumeClaim. Par conséquent, toute personne utilisant l'instance MinIO intégrée à l'Operator a dû effectuer des étapes supplémentaires pour se reconnecter au PersistentVolume précédent contenant les données persistées.

Après la mise à niveau vers l'Operator GitLab `0.6.4`, effectuez les étapes suivantes pour connecter un nouveau PersistentVolumeClaim à un PersistentVolume précédent :

1. Supprimez le Secret `$RELEASE_NAME-minio-secret`. Le contenu du Secret changera avec la mise à niveau vers `0.6.4`, mais son nom restera identique.
1. Modifiez le PersistentVolume MinIO précédent en changeant `.spec.persistentVolumeReclaimPolicy` de `Delete` à `Retain`.
1. Supprimez le StatefulSet MinIO précédent, `$RELEASE_NAME-minio`.
1. Supprimez `.spec.ClaimRef` du PersistentVolume MinIO précédent pour le dissocier du PersistentVolumeClaim MinIO précédent.
1. Supprimez le PersistentVolumeClaim MinIO précédent, `export-gitlab-minio-0`.
1. Confirmez que le statut du PersistentVolume précédent est désormais `Available`.
1. Définissez la valeur suivante dans la ressource personnalisée GitLab : `minio.persistence.volumeName=<nom du PersistentVolume précédent>`.
1. Appliquez la ressource personnalisée GitLab.
1. Supprimez le nouveau PersistentVolumeClaim MinIO (ainsi que le pod MinIO, afin que le PersistentVolumeClaim soit dissocié et puisse être supprimé). L'Operator recréera le PersistentVolumeClaim. Cette étape est nécessaire car le champ `.spec` est immuable.
1. Confirmez que le PersistentVolume MinIO précédent est désormais lié au nouveau PersistentVolumeClaim MinIO.
1. Confirmez que les données sont restaurées en accédant dans l'interface utilisateur GitLab aux tickets, aux artefacts, etc.

Pour plus d'informations sur la reconnexion à des PersistentVolumes précédents, consultez notre [documentation sur les volumes persistants](https://docs.gitlab.com/charts/advanced/persistent-volumes/).

Pour rappel, l'instance MinIO intégrée n'est [pas recommandée pour une utilisation en production](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart).

### Configurer plusieurs connexions à la base de données {#configure-multiple-database-connections}

Dans GitLab 16.0, GitLab utilise par défaut deux connexions à la base de données pointant vers la même base de données PostgreSQL.

Si vous souhaitez revenir à une connexion unique à la base de données, consultez la section [configuration de plusieurs connexions à la base de données](https://docs.gitlab.com/charts/charts/globals/#configure-multiple-database-connections).

### Désactivation ou renommage de composants {#disabling-or-renaming-components}

Bien que le renommage et la désactivation de ressources soient possibles via des modifications de `nameOverride` et la combinaison de diverses valeurs `*.enable: false`, l'Operator GitLab ne supprime pas automatiquement les ressources Kubernetes devenues inutiles. Par conséquent, l'une ou l'autre de ces opérations nécessite une gestion manuelle des ressources résiduelles.

La suppression d'une instance de la ressource personnalisée GitLab, en revanche, supprimera toutes les ressources associées à cette instance, comme attendu.

Le ticket [!889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889) a été créé pour assurer le suivi de ce point.
