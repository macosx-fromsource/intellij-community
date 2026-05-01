---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Mettre à niveau des instances GitLab avec Operator
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

Vous pouvez utiliser GitLab Operator pour mettre à niveau des instances GitLab installées avec GitLab Operator.

## Prérequis {#prerequisites}

Avant de procéder à la mise à niveau avec GitLab Operator :

1. Consultez les [informations nécessaires avant la mise à niveau](https://docs.gitlab.com/update/plan_your_upgrade/).
1. Identifiez la version de GitLab Operator requise pour la version de GitLab souhaitée. Pour connaître les correspondances entre les versions de GitLab, les versions du chart Helm GitLab et les versions de GitLab Operator, consultez les [releases](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases) de GitLab Operator.

## Mettre à niveau GitLab avec GitLab Operator {#upgrade-gitlab-with-gitlab-operator}

Pour mettre à niveau GitLab avec GitLab Operator :

1. Envisagez d'[activer le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/) pendant la mise à niveau afin de restreindre les opérations d'écriture des utilisateurs et d'éviter toute perturbation des workflows.
1. [Mettez à niveau GitLab Runner](https://docs.gitlab.com/runner/install/) vers la même version que votre version cible de GitLab.
1. [Mettez à niveau GitLab Operator](#upgrade-gitlab-operator).
1. [Mettez à niveau GitLab à l'aide de GitLab Operator](#upgrade-gitlab-by-using-gitlab-operator).

Après la mise à niveau :

1. Si activé, [désactivez le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode).
1. Exécutez les [vérifications de l'état de la mise à niveau](https://docs.gitlab.com/update/plan_your_upgrade/#run-upgrade-health-checks).

### Mettre à niveau GitLab Operator {#upgrade-gitlab-operator}

Pour mettre à niveau GitLab Operator :

1. Effectuez une [sauvegarde](https://docs.gitlab.com/charts/backup-restore/).
1. Installez la version requise en utilisant `kubectl` pour appliquer le manifeste correspondant à la version requise de GitLab Operator.

   ```shell
   VERSION=X.Y.Z
   kubectl apply -f \
     https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
   ```

   Cette commande applique toutes les modifications aux manifestes associés, y compris la nouvelle image de déploiement à utiliser.

1. Vérifiez que la nouvelle version de GitLab Operator devient le leader. Le déploiement de GitLab Operator doit créer un nouveau ReplicaSet avec ce changement, ce qui génère un nouveau pod GitLab Operator. Pendant ce temps, le pod GitLab Operator précédent s'arrête et cède son statut de leader. Lorsque cela se produit, le nouveau pod GitLab Operator devient le leader.
1. Mettez à jour la version du chart dans la ressource personnalisée (CR) GitLab. Dans la plupart des cas, les versions de chart disponibles ne sont pas identiques entre les versions de GitLab Operator. Lorsque la nouvelle version de GitLab Operator démarre, elle tente de réconcilier la ressource personnalisée (CR) GitLab existante. Vous pourriez voir une erreur telle que :

   ```plaintext
   Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
   ```

   Pour résoudre ce problème, identifiez une version valide parmi les versions de chart disponibles pour cette release. Par exemple, lors d'une mise à niveau de l'Operator `0.4.0` vers `0.4.1`, mettez à jour la CR GitLab vers une version de chart disponible la plus proche de `5.7.0`, soit `5.7.1` dans ce cas.

1. Vérifiez que GitLab Operator réconcilie GitLab comme prévu. Consultez les journaux du nouveau pod operator pour vérifier que le pod operator a bien été mis à niveau vers la version de chart définie.

   Pour confirmer que la mise à niveau a réussi, obtenez le statut de la CR GitLab :

   ```plaintext
   $ kubectl get gitlabs -n gitlab-system
   NAME     STATUS    VERSION
   gitlab   Running   5.7.1
   ```

   Le statut `Running` indique que GitLab Operator a pu réconcilier les modifications apportées à l'instance. La version doit correspondre à la version du chart spécifiée après la mise à niveau de GitLab Operator.

#### Dépannage {#troubleshooting}

Si vous constatez des erreurs, consultez d'abord notre
[documentation de dépannage](troubleshooting.md).
Si la réponse ne s'y trouve pas, recherchez un ticket existant ou ouvrez-en un nouveau dans notre
[outil de suivi des tickets](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues).

### Mettre à niveau GitLab à l'aide de GitLab Operator {#upgrade-gitlab-by-using-gitlab-operator}

> [!warning]
> Par défaut, GitLab Operator met à niveau GitLab selon l'approche [zéro temps d'arrêt](https://docs.gitlab.com/update/zero_downtime/).
> Par conséquent, GitLab et la version du chart sous-jacent doivent être mis à jour une version mineure à la fois.
>
> Les releases d'Operator antérieures à 2.6.0 et 2.5.1 n'imposaient pas de chemin de mise à niveau valide.
>
> Pour ignorer des versions mineures lors d'une mise à niveau, vous devez [effectuer une mise à niveau avec temps d'arrêt](#upgrade-with-downtime).

1. Mettez à jour le champ `spec.chart.version` dans la ressource personnalisée GitLab vers une nouvelle version. Par exemple :

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   spec:
     chart:
   -   version: "5.0.6"
   +   version: "5.1.1"
       values:
         ...
   ```

1. Appliquez la ressource personnalisée GitLab modifiée au cluster :

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   Vous devriez voir le message suivant :

   ```shell
   gitlab.apps.gitlab.com/gitlab created
   ```

   Vous pouvez suivre la progression dans les journaux du contrôleur. Par exemple :

   ```shell
   $ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
   2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
   2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
   ...
   ```

   Des entrées de journal apparaissent en suivant les étapes de mise à niveau décrites ci-dessus. Vous pouvez également consulter le statut de la ressource personnalisée GitLab dans le cluster :

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS        VERSION
   gitlab   Preparing     5.2.4
   ```

Lorsque l'application est prête et mise à niveau vers la nouvelle version, cela se reflète dans la colonne `STATUS`.

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS      VERSION
gitlab   Running     5.2.4
```

Les conditions de statut sur l'objet GitLab lui-même fournissent des informations plus détaillées sur l'application.

### Mise à niveau avec temps d'arrêt {#upgrade-with-downtime}

Par défaut, GitLab Operator impose un chemin de [mise à niveau zéro temps d'arrêt](https://docs.gitlab.com/update/zero_downtime/), qui nécessite une mise à jour une version mineure à la fois. Si vous préférez ignorer des versions mineures (par exemple, passer de GitLab 18.0 à 18.2), vous pouvez désactiver les mises à niveau zéro temps d'arrêt en ajoutant l'annotation `gitlab.io/disable-zero-downtime-upgrade` à la ressource personnalisée GitLab.

> [!warning]
> Lors d'une mise à niveau avec temps d'arrêt, l'instance GitLab est indisponible pendant l'exécution des migrations de base de données.
> Avant de continuer, assurez-vous d'avoir planifié une fenêtre de maintenance et d'avoir communiqué le temps d'arrêt prévu à vos utilisateurs.
>
> Vous devez toujours respecter les [étapes de mise à niveau obligatoires](https://docs.gitlab.com/update/upgrade_paths/) lors du saut de versions mineures. Planifiez votre chemin de mise à niveau en conséquence pour vous assurer de vous arrêter à chaque version requise avant de passer à votre version cible.

Pour effectuer une mise à niveau avec temps d'arrêt :

1. Envisagez d'[activer le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/).
1. Effectuez une [sauvegarde](https://docs.gitlab.com/charts/backup-restore/).
1. Mettez à niveau GitLab Operator vers une version compatible avec la version cible de GitLab.
1. Ajoutez l'annotation `gitlab.io/disable-zero-downtime-upgrade` et mettez à jour `spec.chart.version` dans la ressource personnalisée GitLab :

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   + annotations:
   +   gitlab.io/disable-zero-downtime-upgrade: "true"
   spec:
     chart:
   -   version: "9.0.0"
   +   version: "9.2.0"
       values:
         ...
   ```

1. Appliquez la ressource personnalisée GitLab modifiée :

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   L'operator effectue automatiquement les opérations suivantes :

   1. Réduit à zéro le nombre de réplicas des déploiements Webservice et Sidekiq.
   1. Attend que tous les pods se terminent.
   1. Exécute les migrations de base de données.
   1. Réconcilie Webservice et Sidekiq avec la nouvelle version du chart.
   1. Restaure le nombre de réplicas de Webservice et Sidekiq à partir des valeurs du chart.

1. Surveillez la progression de la mise à niveau :

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

1. Attendez que la mise à niveau soit terminée :

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS      VERSION
   gitlab   Running     9.3.0
   ```

1. Une fois la mise à niveau terminée, supprimez l'annotation pour rétablir le comportement de mise à niveau zéro temps d'arrêt par défaut pour les mises à niveau futures :

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   - annotations:
   -   gitlab.io/disable-zero-downtime-upgrade: "true"
   spec:
     chart:
       version: "9.3.0"
       values:
         ...
   ```

1. Si activé, [désactivez le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode).

## Fonctionnement des mises à niveau GitLab par GitLab Operator {#how-gitlab-operator-upgrades-gitlab}

Au début de la boucle de réconciliation du contrôleur, GitLab Operator vérifie si la version actuelle correspond à la version requise.

- Si ces versions correspondent, la boucle de réconciliation standard s'exécute, garantissant l'existence des objets satisfaisant la configuration fournie dans la spécification de la ressource personnalisée (CR).
- Si ces versions ne correspondent pas, la boucle de réconciliation standard s'exécute tout de même, mais une branche logique supplémentaire s'exécute pour gérer la mise à niveau.

Lors de la mise à niveau :

1. Le contrôleur réconcilie tous les déploiements. Les déploiements Webservice et Sidekiq sont réconciliés mais mis en pause. Les anciens pods restent actifs jusqu'à la reprise des nouveaux déploiements.
1. Les pré-migrations s'exécutent, ce qui lance le job Migrations en ignorant les migrations post-déploiement.
1. Le contrôleur reprend les déploiements Webservice et Sidekiq.
1. Le contrôleur attend que les nouveaux pods Webservice et Sidekiq soient en cours d'exécution.
1. Les post-migrations s'exécutent, ce qui lance le job Migrations sans ignorer les migrations post-déploiement.
1. Le contrôleur effectue une mise à jour progressive des déploiements Webservice et Sidekiq.
1. Le contrôleur attend que les pods Webservice et Sidekiq redémarrés soient en cours d'exécution.

Dans les boucles de réconciliation suivantes, cette branche logique est ignorée car la version souhaitée (issue de `spec.chart.version`) correspond à la version actuelle (issue de `status.version`).

## Sujets connexes {#related-topics}

- [Mettre à niveau les installations du chart Helm](https://docs.gitlab.com/charts/installation/upgrade/)
- [Versions du chart Helm GitLab](https://docs.gitlab.com/charts/installation/version_mappings/)
- [Planifier votre chemin de mise à niveau](https://docs.gitlab.com/update/upgrade_paths/)
- [Notes de mise à niveau GitLab](https://docs.gitlab.com/update/versions/)
- [Modifications entre les versions de GitLab](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
