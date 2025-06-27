---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLabのアップグレード
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 提供:GitLab Self-Managed

{{< /details >}}

GitLab Operatorは、GitLabのバージョン間のアップグレードを管理できます。このドキュメントでは、アップグレードフローがどのように機能するかの背景となるコンテキストと、GitLabのアップグレードを実行するための手順について説明します。

## OperatorがGitLabのアップグレードを処理する方法 {#how-the-operator-handles-gitlab-upgrades}

コントローラーの調整loopの開始時に、Operatorは現在のバージョンが目的のバージョンと一致するかどうかを確認します。

- これらのバージョンが一致する場合、標準の調整loopが実行され、CR specsで提供された設定を満たすオブジェクトが存在することが保証されます。
- これらのバージョンが一致_しない_場合でも、標準の調整loopは実行されますが、アップグレードフローを処理するために、追加のロジックブランチが実行されます。

アップグレードフローは次のように動作します。

1. コントローラーはすべてのデプロイを調整します。
   - WebserviceとSidekiqのデプロイは調整されますが、「一時停止」されます。つまり、新しいデプロイが一時停止解除されるまで、「古い」ポッドは起動したままになります。
1. 事前移行が実行されます。
   - これにより、移行ジョブが効果的に実行されますが、デプロイ後の移行はスキップされます。
1. コントローラーは、WebserviceとSidekiqのデプロイの一時停止を解除します。
1. コントローラーは、新しいWebserviceとSidekiqのポッドが実行されるのを待ちます。
1. 事後移行が実行されます。
   - これにより、（デプロイ後の移行をスキップせずに）移行ジョブが実行されます。
1. コントローラーは、WebserviceとSidekiqのデプロイでローリングアップデートを実行します。
1. コントローラーは、再起動されたWebserviceとSidekiqのポッドが実行されるのを待ちます。

今後の調整loopでは、目的のバージョン（`spec.chart.version`から）が現在のバージョン（`status.version`から）と一致するため、このロジックブランチはスキップされます。

## GitLabをアップグレードする方法 {#how-to-upgrade-gitlab}

以下は、GitLab Operatorを使用してGitLabインスタンスをアップグレードする手順です。

### ステップ1 {#step-1}

GitLab CRの`spec.chart.version`フィールドを新しいバージョンにアップデートします。次に例を示します。

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

### ステップ2 {#step-2}

変更したGitLab CRをクラスターに適用します。

```shell
kubectl -n gitlab-system apply -f mygitlab.yaml
```

次のメッセージが表示されます。

```shell
gitlab.apps.gitlab.com/gitlab created
```

### ステップ3 {#step-3}

コントローラーlogを介して進行状況を監視できます。

```shell
$ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
...
```

上記のアップグレードフローに従って、logエントリが表示されます。

クラスター内のGitLab CRの状態を表示することもできます。

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS        VERSION
gitlab   Preparing     5.2.4
```

アプリケーションの準備が完了し、新しいバージョンにアップグレードされると、`STATUS`列に反映されます。

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS      VERSION
gitlab   Running     5.2.4
```

GitLabオブジェクト自体の状態は、アプリケーションに関するより詳細な情報を示しています。

## アップグレードに関する追加の考慮事項 {#additional-upgrade-considerations}

以下は、GitLabインスタンスをアップグレードする前に考慮すべき追加のトピックです。

- [PersistentVolumeClaim設定が変更された場合のデータの復元](troubleshooting.md#restoring-data-when-persistentvolumeclaim-configuration-changes):これは、[!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419)がOperator定義のMinIOオブジェクトをGitLab Helm ChartsのMinIOオブジェクトに置き換えた、Operator 0.6.4で特に関連していました。
