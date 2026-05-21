---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: インストール
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

> [!note] GitLab Operatorには[既知の制限](_index.md#known-issues)があり、本番環境での特定のシナリオにのみ適しています。

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->

> [!warning] GitLabカスタムリソースのデフォルト値は**not intended for production use**。これらの値を使用すると、GitLab Operatorは、永続データを含むすべてのサービスがKubernetesクラスターにデプロイされるGitLabインスタンスを作成しますが、これは**本番環境のワークロードには適していません**。本番環境へのデプロイでは、[クラウドネイティブハイブリッドリファレンスアーキテクチャ](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)に従う**必要があります**。GitLabは、Kubernetesクラスター内にデプロイされたPostgreSQL、Redis、Gitaly、Praefect、またはMinIOに関連する問題は一切サポートしません。

このドキュメントでは、KubernetesまたはOpenShiftクラスターでマニフェストを使用してGitLab Operatorをデプロイする方法について説明します。

<!--This warning block is duplicated in `../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml`.
Changes should be reflected in both locations.-->

OpenShiftを使用している場合、インストールは通常、Operator Lifecycle Manager（OLM）によって処理されます。**OLMを使用したインストールは実験的と見なされます**。GitLabは、OLMを使用してデプロイされたインスタンスに関連する問題は一切サポートしません。OLMに関する潜在的な問題の詳細については、[イシュー241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241)を参照してください。

## 前提条件 {#prerequisites}

1. [既存のKubernetesまたはOpenShiftクラスターを使用する、または新たに作成する](#cluster)
1. 前提条件となるサービスとソフトウェアをインストールする
   - [Ingressコントローラー](#ingress-controller)
   - [cert-manager](#tls-certificates)
   - [Metrics Server](#metrics)
1. [ドメイン名サービスを設定する](#configure-domain-name-services)

### クラスター {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

従来のKubernetesクラスターを作成するには、[公式ツール](https://kubernetes.io/docs/tasks/tools/)またはお好みのインストール方法の使用を検討してください。

GitLab Operatorは、次のKubernetesバージョンをサポートしています:

| Kubernetesリリース | ステータス      | 最小Operatorバージョン |
|--------------------|-------------|--------------------------|
| 1.35               | サポート対象   | 2.9.0                    |
| 1.34               | サポート対象   | 2.5.0                    |
| 1.33               | サポート対象   | 2.1.0                    |
| 1.32               | 非推奨  | 2.0.0                    |
| 1.31               | サポート対象外 | 1.9.0                    |

{{< /tab >}}

{{< tab title="OpenShift" >}}

GitLab Operatorは、次のOpenShiftバージョンをサポートしています:

| OpenShiftリリース | ステータス      | 最小Operatorバージョン |
|-------------------|-------------|--------------------------|
| 4.21              | サポート対象   | 2.9.0                    |
| 4.20              | サポート対象   | 2.6.0                    |
| 4.19              | サポート対象   | 2.2.0                    |
| 4.18              | サポート対象   | 1.9.0                    |
| 4.17              | サポート対象外 | 1.6.0                    |

{{< /tab >}}

{{< /tabs >}}

GitLabでは、Kubernetesの直近3つのマイナーバージョンと、OpenShiftの直近4つのマイナーリリースのすべてに対して同時に互換性を維持することを目標としています。新しいバージョンのサポートが追加されると、最も古いサポート対象バージョンのテストは中止されます。当社の目標は、KubernetesとOpenShiftの新しいマイナーリリースが提供開始されてから3か月以内に、Operatorによるサポートを提供することです。

詳細については、[Kubernetesのサポートポリシーを参照してください](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)。

> [!note] [Kubernetes用エージェント](https://docs.gitlab.com/user/clusters/agent/)や[GitLabチャート](https://docs.gitlab.com/charts/installation/cloud/)などの一部のコンポーネントでは、GitLabが異なるクラスターバージョンをサポートする場合があります。

上記のリリースよりも新しいリリースに関する互換性の問題については、[イシュートラッカー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)でご報告ください。

一部のGitLab機能は、非推奨のバージョンや、上記のバージョンよりも古いバージョンでは機能しない場合があります。

Operatorはx86-64とARM64をサポートしています。ARM64ビルドは16.7以降で利用可能ですが、完全なサポートとテストカバレッジの提供は18.8以降です。

### Ingressコントローラー {#ingress-controller}

Ingressコントローラーは、アプリケーションへの外部アクセスを提供し、コンポーネント間のセキュアな通信を確保するために必要です。

GitLab Operatorは、デフォルトで[GitLab HelmチャートからフォークしたNGINXチャート](https://docs.gitlab.com/charts/charts/nginx/)をデプロイします。

外部Ingressコントローラーを使用する場合は、Kubernetesコミュニティの[NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/)を使用してIngressコントローラーをデプロイします。お使いのプラットフォームとお好みのツールに応じて、リンク先の関連手順に従ってください。後で使用するために、Ingressクラスの値を書き留めておいてください（通常、デフォルトは`nginx`です）。GitLab CRを設定する際は、GitLab HelmチャートからのNGINXオブジェクトを無効にするために、必ず`nginx-ingress.enabled=false`を設定してください。

### TLS証明書 {#tls-certificates}

OperatorのKubernetes Webhook用の証明書を作成するには、[cert-manager](https://cert-manager.io)を使用します。GitLabの証明書にも[cert-manager](https://cert-manager.io)を使用してください。

OperatorにはKubernetes Webhook用の証明書が必要なため、GitLabチャートにバンドルされているcert-managerは使用できません。代わりに、Operatorをインストールする前にcert-managerをインストールします。

[インストールドキュメント](https://cert-manager.io/docs/installation/)に従い、プラットフォームとツールで[サポートされているcert-managerリリース](https://cert-manager.io/docs/releases/)をインストールしてください。

### メトリクス {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

HorizontalPodAutoscalersがポッドメトリクスを取得できるように、[Metrics Server](https://github.com/kubernetes-sigs/metrics-server#installation)をインストールします。

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShiftにはデフォルトで[Prometheusアダプター](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html)が付属しているため、GitLab Operatorが別のインスタンスをインストールするのを防ぐには、GitLabカスタムリソースで`spec.chart.values.prometheus.install=false`を設定するだけです。

{{< /tab >}}

{{< /tabs >}}

### ドメイン名サービスを設定する {#configure-domain-name-services}

DNSレコードを追加できる、インターネットからアクセス可能なドメインが必要です。

ドメインをGitLabコンポーネントに接続する方法の詳細については、[ネットワークとDNSのドキュメント](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns)を参照してください。GitLabカスタムリソース（CR）を定義する際に、このセクションで説明されている設定を使用します。

OpenShiftのIngressでは、さらに考慮すべき点があります。詳細については、[OpenShift Ingressに関する注意事項](openshift_ingress.md)を参照してください。

## GitLab Operatorをインストールする {#installing-the-gitlab-operator}

まず、インストール方法を選択します。

{{< tabs >}}

{{< tab title="マニフェスト" >}}

まず、[Operatorのリリースページ](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)からリリースマニフェストを取得します。ターゲットプラットフォームに一致するマニフェストを選択します: KubernetesまたはOpenShift。

次に、Operatorをインストールするネームスペースを作成します。マニフェストでは、ネームスペースはデフォルトで`gitlab-system`に設定されています。ネームスペースを変更するには、マニフェストを手動で更新するか、このキーやその他のキーを簡単に設定できるHelmチャートの使用を検討してください。

```shell
kubectl create namespace gitlab-system
```

最後に、マニフェストを適用します:

```shell
kubectl apply -f gitlab-operator-<platform>.yaml
```

{{< /tab >}}

{{< tab title="Helmチャート" >}}

まず、GitLab Helmリポジトリを追加し、最新のアップデートを取得します。

```shell
helm repo add gitlab https://charts.gitlab.io
helm repo update
```

次に、GitLab Operatorチャートをインストールできます:

```shell
helm install gitlab-operator gitlab/gitlab-operator \
  --create-namespace \
  --namespace gitlab-system
```

利用可能なすべての設定オプションについては、[`values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml)を参照してください。

{{< /tab >}}

{{< tab title="OLM" >}}

GitLab Operatorは、次のOLMチャンネルで使用できます:

- [OperatorHub.io](https://operatorhub.io/operator/gitlab-operator-kubernetes)
- OpenShiftおよびOKDに組み込まれたOperatorHub内の[OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod)
- [Red Hat Ecosystem Catalog](https://catalog.redhat.com/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f)

{{< /tab >}}

{{< /tabs >}}

Operatorデプロイのステータスをチェックして、インストールを確認します:

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

## GitLabをインストールする {#installing-gitlab}

1. GitLabカスタムリソース（CR）を作成します。

   `mygitlab.yaml`などの名前で新しいファイルを作成します。

   このファイルに記述する内容の例を次に示します:

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

   `spec.chart.values`で使用できる設定オプションの詳細については、[GitLab Helmチャートのドキュメント](https://docs.gitlab.com/charts/charts/)を参照してください。

1. 新しいGitLab CRを使用して、GitLabインスタンスをデプロイします。

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   このコマンドは、GitLab Operatorが調整できるように、GitLab CRをクラスターに送信します。コントローラーポッドからのログを追跡して進捗状況を監視できます:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   GitLabリソースを一覧表示してステータスを確認することもできます:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

   CRの調整が完了すると（GitLabリソースのステータスが`Running`になります）、ブラウザで`https://gitlab.example.com`にアクセスしてGitLabを利用できます。

ログインするには、デプロイの初期ルートパスワードを取得する必要があります。詳細な手順については、[Helmチャートのドキュメント](https://docs.gitlab.com/charts/installation/deployment/#initial-login)を参照してください。

## 推奨される次のステップ {#recommended-next-steps}

インストールが完了したら、[推奨される次のステップ](https://docs.gitlab.com/install/next_steps/)（認証オプションやサインアップ制限など）を実行することを検討してください。

### OpenShift {#openshift}

OpenShiftを実行している場合は、GitLab Operatorの承認戦略を自動（デフォルト）から手動に変更してください。これにより、[承認が得られる](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators)まで、OpenShiftが新しいOperatorバージョンをインストールできなくなります。

Operatorのバージョンを固定する、または最新以外のバージョンにアップグレードするため、カスタム[`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster)を設定することもできます。

- 承認戦略は、[OpenShift Webコンソール](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf)から変更するか、[サブスクリプションを編集する](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm)ことで変更できます。
- 手動アップグレードを承認するには、`InstallPlan`の`.spec.approved`を`true`に設定します。
- 各GitLab Operatorは、特定のGitLabチャートバージョンのサブセットのみをサポートしています。そのため、GitLab Operatorをアップグレードする場合は、GitLabカスタムリソース内のチャートバージョンの更新も併せて行う必要があります。
- GitLab Operatorと指定されたGitLab Helmチャートバージョンに互換性がない場合、[GitLab Helmチャートバージョンに関するエラー](gitlab_upgrades.md)によりチャートの設定変更が失敗することがあります。

> [!note] [OLMはOperatorのダウングレードをサポートしていません](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177)。

## GitLab Operatorをアンインストールする {#uninstall-the-gitlab-operator}

GitLab Operatorとその関連リソースを削除するには、次の手順に従います。

Operatorをアンインストールする前に、次の点に注意してください:

- GitLabインスタンスを削除しても、OperatorはPersistentVolumeClaimやシークレットを削除しません。
- Operatorを削除しても、インストール先のネームスペース（デフォルトでは`gitlab-system`）は自動的には削除されません。これは、永続ボリュームが誤って失われることを防ぐためです。

### GitLabインスタンスをアンインストールする {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

これにより、GitLabインスタンスとすべての関連オブジェクトが削除されます（前述のとおり、PersistentVolumeClaimを除きます）。

### GitLab Operatorをアンインストールする {#uninstall-the-gitlab-operator-1}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

これにより、実行中のOperatorのデプロイを含む、Operatorのリソースが削除されます。GitLabインスタンスに関連付けられたオブジェクトは削除**されません**。

## GitLab Operatorのトラブルシューティング {#troubleshoot-the-gitlab-operator}

GitLab Operatorのトラブルシューティングに関する情報は、[トラブルシューティング](troubleshooting.md)を参照してください。
