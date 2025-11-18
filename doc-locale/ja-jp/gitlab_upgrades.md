---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Operatorを使用したGitLabインスタンスのアップグレード
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

GitLab Operatorを使用すると、GitLab Operatorでインストールされたインスタンスをアップグレードできます。

## 前提要件 {#prerequisites}

GitLab Operatorでアップグレードする前に:

1. [アップグレードする前に必要な情報](https://docs.gitlab.com/update/plan_your_upgrade/)を確認してください。
1. 必要なGitLabのバージョンに必要なGitLab Operatorのバージョンを特定します。GitLabのバージョン、GitLab Helmチャートのバージョン、およびGitLab Operatorのバージョン間のマッピングについては、GitLab Operatorの[リリース](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)を参照してください。

## GitLab Operatorを使用したGitLabのアップグレード {#upgrade-gitlab-with-gitlab-operator}

GitLab Operatorを使用してGitLabをアップグレードするには:

1. ワークフローを妨げないように、アップグレード中に[メンテナンスモードをオンにすること](https://docs.gitlab.com/administration/maintenance_mode/)を検討して、書き込み操作からユーザーを制限します。
1. ターゲットのGitLabバージョンと同じバージョンに[GitLab Runner](https://docs.gitlab.com/runner/install/)をアップグレードします。
1. [GitLab Operator](#upgrade-gitlab-operator)をアップグレードします。
1. [GitLab Operator](#upgrade-gitlab-by-using-gitlab-operator)を使用してGitLabをアップグレードします。

アップグレード後:

1. 有効になっている場合は、[メンテナンスモードをオフにします](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode)。
1. [アップグレードヘルスチェック](https://docs.gitlab.com/update/plan_your_upgrade/#run-upgrade-health-checks)を実行します。

### GitLab Operatorのアップグレード {#upgrade-gitlab-operator}

GitLab Operatorをアップグレードするには:

1. [バックアップ](https://docs.gitlab.com/charts/backup-restore/)を実行します。
1. `kubectl`を使用して、必要なGitLab Operatorのバージョンのマニフェストを適用して、必要なバージョンをインストールします。

   ```shell
   VERSION=X.Y.Z
   kubectl apply -f \
     https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
   ```

   このコマンドは、使用する新しいデプロイイメージを含む、関連するマニフェストに対する変更を適用します。

1. 新しいGitLab Operatorのバージョンがリーダーになることを確認します。GitLab Operatorのデプロイは、この変更により新しいReplicaSetを作成し、新しいGitLab Operatorポッドを起動する必要があります。一方、以前のGitLab Operatorポッドがシャットダウンし、リーダーのステータスを放棄します。これが発生すると、新しいGitLab Operatorポッドがリーダーになります。
1. GitLabカスタムリソース(CR)のチャートバージョンを更新します。ほとんどの場合、使用可能なチャートのバージョンは、GitLab Operatorのバージョン間で同一ではありません。新しいバージョンのGitLab Operatorが起動すると、既存のGitLabカスタムリソース（CR）を調整しようとします。次のようなエラーが表示されることがあります:

   ```plaintext
   Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
   ```

   これに対処するには、そのリリースの使用可能なチャートバージョンから有効なバージョンを特定します。たとえば、Operator `0.4.0`から`0.4.1`にアップグレードする場合は、GitLab CRを`5.7.0`に最も近い使用可能なチャートバージョンに更新します。この場合は`5.7.1`です。

1. GitLab OperatorがGitLabを期待どおりに調整することを確認します。新しいOperatorポッドからのログをチェックして、Operatorポッドが定義されたチャートのバージョンにアップグレードされたかどうかを確認します。

   アップグレードが成功したことを確認するには、GitLab CRのステータスを取得します:

   ```plaintext
   $ kubectl get gitlabs -n gitlab-system
   NAME     STATUS    VERSION
   gitlab   Running   5.7.1
   ```

   ステータス`Running`は、GitLab Operatorがインスタンスへの変更を調整できることを意味します。バージョンは、GitLab Operatorのアップグレード後に指定されたチャートのバージョンと一致する必要があります。

#### トラブルシューティング {#troubleshooting}

エラーが発生した場合は、まず[トラブルシューティング](troubleshooting.md)ドキュメントを参照してください。そこに回答がない場合は、既存のイシューを確認するか、[イシュートラッカー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)で新しいイシューを開いてください。

### GitLab Operatorを使用したGitLabのアップグレード {#upgrade-gitlab-by-using-gitlab-operator}

GitLab Operatorをアップグレードした後、GitLabインスタンスをアップグレードするには:

1. GitLabカスタムリソースの`spec.chart.version`フィールドを新しいバージョンに更新します。例: 

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

1. 変更されたGitLabカスタムリソースをクラスターに適用します:

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   次のメッセージが表示されます:

   ```shell
   gitlab.apps.gitlab.com/gitlab created
   ```

   コントローラーログで進行状況を監視できます。例: 

   ```shell
   $ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
   2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
   2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
   ...
   ```

   上記のアップグレード手順に従ってログエントリが表示されます。クラスター内のGitLabカスタムリソースステータスを表示することもできます:

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

GitLabオブジェクト自体のステータス状態は、アプリケーションに関するより詳細な情報を示しています。

## GitLab OperatorがGitLabをアップグレードする方法 {#how-gitlab-operator-upgrades-gitlab}

コントローラー調整ループの開始時に、GitLab Operatorは、現在のバージョンが必要なバージョンと一致するかどうかを確認します。

- これらのバージョンが一致する場合、通常の調整ループが実行され、カスタムリソース（CR）仕様で提供される設定を満たすオブジェクトが存在することを確認します。
- これらのバージョンが一致しない場合でも、通常の調整ループは実行されますが、アップグレードを処理するためのロジックの追加ブランチが実行されます。

アップグレードでは:

1. コントローラーは、すべてのデプロイを調整します。WebserviceとSidekiqのデプロイは調整されますが、一時停止されます。古いポッドは、新しいデプロイが再開されるまで起動したままになります。
1. 事前移行が実行され、移行ジョブが実行されますが、デプロイ後の移行はスキップされます。
1. コントローラーは、WebserviceとSidekiqのデプロイを再開します。
1. コントローラーは、新しいWebserviceおよびSidekiqポッドが実行されるのを待ちます。
1. デプロイ後の移行が実行され、デプロイ後の移行をスキップせずに移行ジョブが実行されます。
1. コントローラーは、WebserviceとSidekiqのデプロイに対してローリングアップデートを実行します。
1. コントローラーは、再起動されたWebserviceおよびSidekiqポッドが実行されるのを待ちます。

将来の調整ループでは、（`spec.chart.version`から）目的のバージョンが（`status.version`から）現在のバージョンと一致するため、このロジックのブランチはスキップされます。

## 関連トピック {#related-topics}

- [Helmチャートインストールをアップグレード](https://docs.gitlab.com/charts/installation/upgrade/)
- [GitLab Helm chart](https://docs.gitlab.com/charts/installation/version_mappings/)のバージョン
- [アップグレードパスを計画する](https://docs.gitlab.com/update/upgrade_paths/)
- [GitLabアップグレードノート](https://docs.gitlab.com/update/versions/)
- [GitLabのバージョン間の変更点](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
