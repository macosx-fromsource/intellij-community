---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Dépanner l'opérateur GitLab"
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

Ce document est un recueil de notes et de conseils pour faciliter le dépannage de l'installation de l'opérateur GitLab et du déploiement d'une instance GitLab à partir de la ressource personnalisée GitLab.

## Problèmes d'installation {#installation-problems}

Le dépannage de l'installation de l'opérateur GitLab dans un environnement Kubernetes ressemble beaucoup au dépannage de toute autre charge de travail Kubernetes. Après avoir déployé le manifeste de l'opérateur GitLab, surveillez les données de sortie de `kubectl describe` pour le pod de l'opérateur GitLab ou de `kubectl get events -n <namespace>`. Cela indiquera tout problème lié à la récupération de l'image de l'opérateur GitLab ou à toute autre condition préalable au démarrage de l'opérateur GitLab.

Si l'opérateur GitLab démarre mais se ferme prématurément, l'examen des logs de l'opérateur GitLab peut fournir des informations pour déterminer la cause de la fin du pod. Cela peut être effectué avec la commande suivante :

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

De plus, l'opérateur GitLab dépend de CertManager pour créer un certificat TLS afin de fonctionner correctement. Le certificat TLS est créé en tant que secret et monté en tant que volume sur le pod de l'opérateur GitLab. Les problèmes liés à l'obtention du certificat TLS peuvent être trouvés dans le journal des événements de l'espace de nommage.

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

L'étape suivante consiste à inspecter les logs de CertManager à la recherche de problèmes qui indiquent l'échec de la création du certificat TLS.

### Problèmes spécifiques à OpenShift {#openshift-specific-problems}

OpenShift dispose d'un modèle de sécurité plus restrictif et, par conséquent, l'opérateur GitLab doit être installé avec le compte d'administrateur du cluster. Les comptes de développeur ne disposent pas des privilèges nécessaires pour permettre à l'opérateur GitLab de fonctionner correctement.

Si les pods du [contrôleur Ingress NGINX](https://github.com/kubernetes/ingress-nginx) ne peuvent pas être provisionnés en raison de paramètres de contraintes de contexte de sécurité (SCC) invalides comme décrit dans [ce ticket](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762), la solution de contournement appropriée est de mettre à jour les SCC depuis le dépôt pour permettre le démarrage d'Ingress NGINX dans votre cluster OpenShift :

1. Récupérez le [dernier manifeste OpenShift](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases) pour l'opérateur GitLab. Vous avez besoin de `gitlab-operator-openshift-VERSION.yaml`
1. Extrayez les SCC :

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. Appliquez `scc.yaml` à votre cluster :

   ```shell
   kubectl apply -f scc.yaml
   ```

L'installation à partir d'un manifeste publié depuis la page des releases du dépôt de l'opérateur GitLab ne présentera pas ce problème, car les SCC sont incluses. Voici les tickets connexes liés aux objets non pris en charge dans les OperatorHubs :

- [Ajouter une prise en charge pour la ressource personnalisée pour IngressClass · Ticket #5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [Ajouter une prise en charge pour les contraintes de contexte de sécurité OpenShift · Ticket #2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## Problèmes de déploiement de l'instance GitLab {#problems-with-deployment-of-gitlab-instance}

En plus des informations présentées ici, il convient de consulter la [documentation de dépannage](https://docs.gitlab.com/charts/troubleshooting/) du chart Helm GitLab.

### Services principaux non prêts {#core-services-not-ready}

L'opérateur GitLab repose sur l'installation d'instances de Redis, PostgreSQL et Gitaly, connus sous le nom de services principaux. Si, après le déploiement d'une ressource personnalisée GitLab, un nombre excessif de messages de log de l'opérateur GitLab indique que les services principaux ne sont pas prêts, l'un de ces services a des difficultés à devenir opérationnel.

Vérifiez spécifiquement les points de terminaison de chacun de ces services pour vous assurer qu'ils sont connectés au pod du service. Cela peut également indiquer que le cluster ne dispose pas de suffisamment de ressources pour prendre en charge l'instance GitLab et que des nœuds supplémentaires doivent être ajoutés au cluster.

Le ticket [305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305) a été créé pour suivre le signalement du service principal qui bloque le déploiement de l'instance GitLab.

### Interface utilisateur GitLab inaccessible (les Ingresses n'ont pas d'adresse et/ou les Challenges CertManager échouent) {#gitlab-ui-unreachable-ingresses-have-no-address-andor-certmanager-challenges-failing}

Le manifeste d'installation et le Helm Chart de l'opérateur GitLab utilisent `gitlab` comme préfixe pour tous les noms de ressources par défaut, sauf si `nameOverride` est précisé dans les valeurs Helm.

En conséquence, l'IngressClass NGINX sera nommée `gitlab-nginx`. Si un nom de release autre que `gitlab` est indiqué dans la ressource personnalisée GitLab sous `metadata.name`, alors le nom d'IngressClass par défaut doit être défini explicitement sous `global.ingress.class` :

Par exemple : si `metadata.name` est défini sur `demo`, alors définissez `global.ingress.class=gitlab-nginx` :

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

Dans un environnement OpenShift, le [contrôleur Ingress NGINX](https://kubernetes.github.io/ingress-nginx/) est utilisé à la place des routes OpenShift pour diriger le trafic vers l'instance GitLab (HTTPS et SSH). Si vous rencontrez un problème de connexion à l'instance GitLab, assurez-vous d'abord qu'il existe un déploiement pour le contrôleur Ingress NGINX.

Si un déploiement est présent, vérifiez la colonne `READY` de la sortie de `kubectl get deploy`. Si le statut `READY` est renvoyé comme `0/0`, inspectez la sortie de `kubectl get events -n <namespace> | grep -i nginx` en recherchant les messages indiquant que la SCC a été enfreinte.

Cela indique que les ressources de contrôle d'accès basé sur les rôles (RBAC) NGINX pour OpenShift n'ont pas été déployées. Le manifeste de l'opérateur GitLab pour OpenShift doit être réappliqué avec la commande suivante :

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

Une fois le manifeste appliqué, il peut être nécessaire de supprimer le déploiement du contrôleur Ingress afin d'acquérir correctement la SCC et de permettre au contrôleur Ingress de créer les pods correctement.

### Les autoscalers horizontaux de pods ne se mettent pas à l'échelle {#horizontal-pod-autoscalers-are-not-scaling}

S'il s'avère que les autoscalers horizontaux de pods (HPA) ne mettent pas à l'échelle le nombre de pods en fonction de la charge de trafic, vérifiez l'installation du serveur de métriques. Dans un cluster Kubernetes, le serveur de métriques est un composant supplémentaire qui doit être installé. Le processus d'installation est décrit dans la [documentation d'installation](installation.md#metrics).

Un cluster OpenShift dispose d'un serveur de métriques intégré et, par conséquent, les HPA devraient fonctionner correctement.

### Restauration des données lors de modifications de la configuration de demandes de volume persistant {#restoring-data-when-persistentvolumeclaim-configuration-changes}

Lorsque vous travaillez avec des composants tels que MinIO pour la persistance des données, il peut parfois être nécessaire de se reconnecter à un volume persistant précédent.

Par exemple, [la merge request 419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419) a remplacé les composants MinIO définis par l'opérateur GitLab par les composants MinIO des GitLab Helm Charts. Dans le cadre de cette modification, les noms des objets ont changé, y compris la demande de volume persistant. En conséquence, toute personne utilisant l'instance MinIO intégrée à l'opérateur GitLab a dû effectuer des étapes supplémentaires pour se reconnecter au volume persistant précédent contenant les données persistées.

Après la mise à niveau vers GitLab Operator `0.6.4`, effectuez les étapes suivantes pour connecter une nouvelle demande de volume persistant à un volume persistant précédent :

1. Supprimez le secret `$RELEASE_NAME-minio-secret`. Le contenu du secret changera avec la mise à niveau vers `0.6.4`, mais pas le nom du secret.
1. Modifiez le volume persistant MinIO précédent en remplaçant `.spec.persistentVolumeReclaimPolicy` de `Delete` par `Retain`.
1. Supprimez le StatefulSet MinIO précédent, `$RELEASE_NAME-minio`.
1. Supprimez `.spec.ClaimRef` du volume persistant MinIO précédent pour le dissocier de la demande de volume persistant MinIO précédente.
1. Supprimez la demande de volume persistant MinIO précédente, `export-gitlab-minio-0`.
1. Confirmez que le statut du volume persistant précédent est maintenant `Available`.
1. Définissez la valeur suivante dans la ressource personnalisée GitLab : `minio.persistence.volumeName=<previous PersistentVolume name>`.
1. Appliquez la ressource personnalisée GitLab.
1. Supprimez la nouvelle demande de volume persistant MinIO (et le pod MinIO, afin que la demande de volume persistant soit dissociée et puisse être supprimée). L'opérateur GitLab recréera la demande de volume persistant. Cela est nécessaire car le champ `.spec` est immuable.
1. Confirmez que le volume persistant MinIO précédent est maintenant lié à la nouvelle demande de volume persistant MinIO.
1. Confirmez que les données sont restaurées en naviguant dans l'interface utilisateur GitLab vers les tickets, les artefacts, etc.

Pour plus d'informations sur la reconnexion aux volumes persistants précédents, consultez notre [documentation sur les volumes persistants](https://docs.gitlab.com/charts/advanced/persistent-volumes/).

Pour rappel, l'instance MinIO intégrée n'est [pas recommandée pour une utilisation en production](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart).

### Configurer plusieurs connexions de base de données {#configure-multiple-database-connections}

Dans GitLab 16.0, GitLab utilise par défaut deux connexions de base de données pointant vers la même base de données PostgreSQL.

Si vous souhaitez revenir à une connexion de base de données unique, reportez-vous à la [configuration de plusieurs connexions de base de données](https://docs.gitlab.com/charts/charts/globals/#configure-multiple-database-connections).

### Désactivation ou renommage des composants {#disabling-or-renaming-components}

Bien que le renommage et la désactivation des ressources soient possibles via des modifications de `nameOverride` et la combinaison de diverses valeurs `*.enable: false`, l'opérateur GitLab ne supprime pas automatiquement les ressources Kubernetes qui ne sont plus nécessaires. En conséquence, l'une ou l'autre des opérations ci-dessus nécessiterait une gestion manuelle des ressources restantes.

La suppression d'une instance de la ressource personnalisée GitLab, en revanche, supprimera toutes les ressources associées à cette instance comme prévu.

Le ticket [889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889) a été créé pour en assurer le suivi.
