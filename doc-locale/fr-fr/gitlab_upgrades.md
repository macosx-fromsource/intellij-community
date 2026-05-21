---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Mettre à niveau les instances GitLab avec l'opérateur GitLab
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

Vous pouvez utiliser l'opérateur GitLab pour mettre à niveau les instances GitLab qui ont été installées avec l'opérateur GitLab.

## Prérequis {#prerequisites}

Avant de procéder à la mise à niveau avec l'opérateur GitLab :

1. Consultez les [informations dont vous avez besoin](https://docs.gitlab.com/update/plan_your_upgrade/).
1. Identifiez la version de l'opérateur GitLab requise pour la version de GitLab souhaitée. Pour connaître les correspondances entre les versions de GitLab, les versions du chart Helm GitLab et les versions de l'opérateur GitLab, consultez les [releases](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases) de l'opérateur GitLab.

## Mettre à niveau GitLab avec l'opérateur GitLab {#upgrade-gitlab-with-gitlab-operator}

Pour mettre à niveau GitLab avec l'opérateur GitLab :

1. Pensez à [activer le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/) pendant la mise à niveau pour restreindre les opérations d'écriture des utilisateurs et éviter de perturber les workflows.
1. [Mettez à niveau GitLab Runner](https://docs.gitlab.com/runner/install/) vers la même version que votre version cible de GitLab.
1. [Mettez à niveau l'opérateur GitLab](#upgrade-gitlab-operator).
1. [Mettez à niveau GitLab en utilisant l'opérateur GitLab](#upgrade-gitlab-by-using-gitlab-operator).

Après la mise à niveau :

1. S'il est activé, [désactivez le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode).
1. Vérifiez [l'état de la mise à niveau](https://docs.gitlab.com/update/plan_your_upgrade/#run-upgrade-health-checks).

### Mettre à niveau l'opérateur GitLab {#upgrade-gitlab-operator}

Pour mettre à niveau l'opérateur GitLab :

1. Effectuez une [sauvegarde](https://docs.gitlab.com/charts/backup-restore/).
1. Installez la version requise en utilisant `kubectl` pour appliquer le manifeste de la version requise de l'opérateur GitLab.

   ```shell
   VERSION=X.Y.Z
   kubectl apply -f \
     https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
   ```

   Cette commande applique toutes les modifications aux manifestes associés, y compris la nouvelle image de déploiement à utiliser.

1. Vérifiez que la nouvelle version de l'opérateur GitLab a obtenu le rôle de leader. Le déploiement de l'opérateur GitLab doit créer un nouveau ReplicaSet avec cette modification, ce qui crée un nouveau pod de l'opérateur GitLab. Pendant ce temps, le pod de l'opérateur GitLab précédent s'arrête et abandonne son statut de leader. Lorsque cela se produit, le nouveau pod de l'opérateur GitLab obtient le rôle de leader.
1. Mettez à jour la version du chart dans la ressource personnalisée GitLab. Dans la plupart des cas, les versions de chart disponibles ne sont pas identiques entre les versions de l'opérateur GitLab. Lorsque la version plus récente de l'opérateur GitLab démarre, elle tente de réconcilier la ressource personnalisée GitLab existante. Vous pourriez voir une erreur telle que :

   ```plaintext
   Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
   ```

   Pour y remédier, identifiez une version valide parmi les versions de chart disponibles pour cette release. Par exemple, lors de la mise à niveau de l'opérateur GitLab `0.4.0` vers `0.4.1`, mettez à jour la ressource personnalisée GitLab vers une version de chart disponible la plus proche de `5.7.0`, qui dans ce cas est `5.7.1`.

1. Vérifiez que l'opérateur GitLab réconcilie GitLab comme prévu. Consultez les logs du nouveau pod de l'opérateur pour vérifier si le pod de l'opérateur GitLab a été mis à niveau vers la version de chart définie.

   Pour confirmer que la mise à niveau a réussi, obtenez le statut de la ressource personnalisée GitLab :

   ```plaintext
   $ kubectl get gitlabs -n gitlab-system
   NAME     STATUS    VERSION
   gitlab   Running   5.7.1
   ```

   Le statut `Running` indique que l'opérateur GitLab a pu réconcilier les modifications apportées à l'instance. La version doit correspondre à la version du chart indiquée après la mise à niveau de l'opérateur GitLab.

#### Dépannage {#troubleshooting}

Si vous remarquez des erreurs, consultez d'abord notre [documentation](troubleshooting.md). Si la réponse n'y figure pas, recherchez un ticket existant ou ouvrez un nouveau ticket dans notre [système de suivi](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues).

### Mettre à niveau GitLab en utilisant l'opérateur GitLab {#upgrade-gitlab-by-using-gitlab-operator}

> [!warning] 
> Par défaut, l'opérateur GitLab met à niveau GitLab en utilisant l'approche [sans temps d'arrêt](https://docs.gitlab.com/update/zero_downtime/). Par conséquent, GitLab et la version de chart sous-jacente doivent être mis à jour une version mineure à la fois.
>
> Les releases de l'opérateur GitLab antérieures à 2.6.0 et 2.5.1 n'imposaient pas un chemin de mise à niveau valide.
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

   Vous pouvez suivre la progression dans les logs du contrôleur. Par exemple :

   ```shell
   $ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
   2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
   2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
   ...
   ```

   Vous verrez des entrées dans les logs suivant les étapes de mise à niveau décrites ci-dessus. Vous pouvez également afficher le statut de la ressource personnalisée GitLab dans le cluster :

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS        VERSION
   gitlab   Preparing     5.2.4
   ```

Lorsque l'application est prête et mise à niveau vers la nouvelle version, la colonne `STATUS` l'indiquera.

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS      VERSION
gitlab   Running     5.2.4
```

Les conditions de statut sur l'objet GitLab lui-même présentent des informations plus détaillées sur l'application.

### Mise à niveau avec temps d'arrêt {#upgrade-with-downtime}

Par défaut, l'opérateur GitLab applique un chemin de [mise à niveau sans temps d'arrêt](https://docs.gitlab.com/update/zero_downtime/), qui nécessite de mettre à jour une version mineure à la fois. Si vous préférez ignorer des versions mineures (par exemple, mettre à niveau de GitLab 18.0 à 18.2), vous pouvez désactiver les mises à niveau sans temps d'arrêt en ajoutant l'annotation `gitlab.io/disable-zero-downtime-upgrade` à la ressource personnalisée GitLab.

> [!warning] 
> Lors d'une mise à niveau avec temps d'arrêt, l'instance GitLab est indisponible pendant l'exécution des migrations de base de données. Avant de continuer, assurez-vous d'avoir planifié une fenêtre de maintenance et d'avoir communiqué le temps d'arrêt prévu à vos utilisateurs.
>
> Vous devez tout de même respecter les [versions intermédiaires requises dans la mise à niveau](https://docs.gitlab.com/update/upgrade_paths/) lorsque vous ignorez des versions mineures. Planifiez votre chemin de mise à niveau en conséquence pour vous assurer de mettre à niveau vers chaque version requise avant de passer à votre version cible.

Pour effectuer une mise à niveau avec temps d'arrêt :

1. Pensez à [activer le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/).
1. Effectuez une [sauvegarde](https://docs.gitlab.com/charts/backup-restore/).
1. Mettez à niveau l'opérateur GitLab vers une version qui prend en charge la version cible de GitLab.
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

   L'opérateur effectue automatiquement les opérations suivantes :

   1. Réduire les déploiements Webservice et Sidekiq à zéro répliques.
   1. Attendre que tous les pods se terminent.
   1. Exécuter les migrations de base de données.
   1. Réconcilier Webservice et Sidekiq avec la nouvelle version du chart.
   1. Restaurer le nombre de répliques Webservice et Sidekiq à partir des valeurs du chart.

1. Surveillez la progression de la mise à niveau :

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

1. Attendez la fin de la mise à niveau :

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS      VERSION
   gitlab   Running     9.3.0
   ```

1. Une fois la mise à niveau terminée, supprimez l'annotation pour rétablir le comportement de mise à niveau sans temps d'arrêt par défaut pour les futures mises à niveau :

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

1. S'il est activé, [désactivez le mode maintenance](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode).

## Fonctionnement des mises à niveau GitLab par l'opérateur GitLab {#how-gitlab-operator-upgrades-gitlab}

Au début de la boucle de réconciliation du contrôleur, l'opérateur GitLab vérifie si la version actuelle correspond à la version requise.

- Si ces versions correspondent, la boucle de réconciliation normale s'exécute et garantit que les objets satisfaisant la configuration fournie dans la spécification de la ressource personnalisée existent.
- Si ces versions ne correspondent pas, la boucle de réconciliation normale s'exécute quand même, mais une branche logique supplémentaire s'exécute pour gérer la mise à niveau.

Lors de la mise à niveau :

1. Le contrôleur réconcilie tous les déploiements. Les déploiements Webservice et Sidekiq sont réconciliés mais sont mis en pause. Les anciens pods restent actifs jusqu'à ce que les nouveaux déploiements soient repris.
1. Les pré-migrations s'exécutent, ce qui lance le job Migrations, mais ignore les migrations post-déploiement.
1. Le contrôleur reprend les déploiements Webservice et Sidekiq.
1. Le contrôleur attend que les nouveaux pods Webservice et Sidekiq soient en cours d'exécution.
1. Les post-migrations s'exécutent, ce qui lance le job Migrations sans ignorer les migrations post-déploiement.
1. Le contrôleur effectue une mise à jour progressive des déploiements Webservice et Sidekiq.
1. Le contrôleur attend que les pods Webservice et Sidekiq redémarrés soient en cours d'exécution.

Dans les futures boucles de réconciliation, cette branche logique est ignorée, car la version souhaitée (issue de `spec.chart.version`) correspond à la version actuelle (issue de `status.version`).

## Sujets connexes {#related-topics}

- [Mettre à niveau les installations du chart Helm](https://docs.gitlab.com/charts/installation/upgrade/)
- [Versions du chart Helm GitLab](https://docs.gitlab.com/charts/installation/version_mappings/)
- [Planifier votre chemin de mise à niveau](https://docs.gitlab.com/update/upgrade_paths/)
- [Notes de mise à niveau GitLab](https://docs.gitlab.com/update/versions/)
- [Modifications entre les versions de GitLab](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
