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

{{< alert type="note" >}}

GitLab Operatorには[既知の制限事項](_index.md#known-issues)があり、本番環境での使用における特定のシナリオにのみ適しています。

{{< /alert >}}

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->

{{< alert type="warning" >}}

GitLabカスタムリソースのデフォルト値は、**not intended for production use**。これらの値を使用すると、GitLab Operatorは、永続データを含むすべてのサービスがKubernetesクラスターにデプロイされるGitLabインスタンスを作成します。これは、**not suitable for production workloads**。本番環境へのデプロイでは、[クラウドネイティブハイブリッドリファレンスアーキテクチャ](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)に従う**必要があります**。GitLabは、Kubernetesクラスター内にデプロイされたPostgreSQL、Redis、Gitaly、Praefect、またはMinIOに関連するイシューをサポートしません。

{{< /alert >}}

このドキュメントでは、KubernetesまたはOpenShiftクラスターでマニフェストを使用してGitLab Operatorをデプロイする方法について説明します。

<!--This warning block is duplicated in ../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml.
Changes should be reflected in both locations.-->

OpenShiftを使用している場合、通常、インストールはOperator Lifecycle Manager（OLM）によって処理されます。**Installation using OLM is considered experimental**。GitLabは、OLMを使用してデプロイされたインスタンスに関連するイシューをサポートしていません。OLMの潜在的なイシューの詳細については、[イシュー241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241)を参照してください。

## 前提条件 {#prerequisites}

1. [既存のKubernetesまたはOpenShiftクラスターを作成または使用する](#cluster)
1. 前提条件となるサービスとソフトウェアをインストールする
   - [Ingressコントローラー](#ingress-controller)
   - [cert-manager](#tls-certificates)
   - [メトリクスサーバー](#metrics)
1. [ドメインネームサービスを設定する](#configure-domain-name-services)

### クラスター {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

従来のKubernetesクラスターを作成するには、[公式ツール](https://kubernetes.io/docs/tasks/tools/)または推奨されるインストール方法の使用を検討してください。

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

Kubernetesの最新のマイナーバージョン3つと、OpenShiftの最新のマイナーリリース4つとの互換性を目標としています。新しいバージョンのサポートが追加されると、サポートされている最も古いバージョンのテストは中止されます。私たちの目標は、KubernetesとOpenShiftの新しいマイナーリリースの初期可用性から3か月以内に、Operatorのサポートを提供することです。

詳細については、[Kubernetesのサポートポリシーを参照してください](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)。

注: [Kubernetes](https://docs.gitlab.com/user/clusters/agent/)用エージェントや[GitLabチャート](https://docs.gitlab.com/charts/installation/cloud/)など、一部のコンポーネントでは、GitLabが異なるクラスターバージョンをサポートしている場合があります。

上記のリリースよりも新しいリリースに関する互換性のイシューについては、[イシュートラッカー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)でお知らせください。

一部のGitLab機能は、非推奨のバージョンや、上記のバージョンよりも古いバージョンでは機能しない場合があります。

Operatorはx86-64とARM64をサポートしています。ARM64ビルドは16.7から利用可能ですが、完全なサポートとテストカバレッジは18.8から利用可能です。

### Ingressコントローラー {#ingress-controller}

Ingressコントローラーは、アプリケーションへの外部アクセスを提供し、コンポーネント間のセキュアな通信を確保するために必要です。

GitLab Operatorは、デフォルトで[フォークしたNGINXチャートをGitLab Helmチャートからデプロイします](https://docs.gitlab.com/charts/charts/nginx/)。

外部Ingressコントローラーを使用する場合は、Kubernetesコミュニティの[NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/)を使用してIngressコントローラーをデプロイします。プラットフォームと推奨されるツールに基づいて、リンク内の関連する手順に従ってください。後で使用するために、Ingressクラスの値に注意してください（通常、デフォルトは`nginx`です）。GitLab CRを設定する場合は、GitLab HelmチャートからNGINXオブジェクトを無効にするために、必ず`nginx-ingress.enabled=false`を設定してください。

### TLS証明書 {#tls-certificates}

OperatorのKubernetes Webhookの証明書を作成するために、[cert-manager](https://cert-manager.io)が使用されます。GitLab証明書にも[cert-manager](https://cert-manager.io)を使用する必要があります。

OperatorにはKubernetes Webhookの証明書が必要なため、GitLabチャートにバンドルされているcert-managerは使用できません。代わりに、Operatorをインストールする前にcert-managerをインストールします。

プラットフォームとツールに対応する[サポートされているcert-managerリリース](https://cert-manager.io/docs/releases/)をインストールするには、[インストールドキュメント](https://cert-manager.io/docs/installation/)に従ってください。

### メトリクス {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

HorizontalPodAutoscalersがポッドメトリクスを取得できるように、[メトリクスサーバー](https://github.com/kubernetes-sigs/metrics-server#installation)をインストールします。

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShiftにはデフォルトで[Prometheusアダプター](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html)が付属しているため、GitLabカスタムリソースで`spec.chart.values.prometheus.install=false`を設定して、GitLab Operatorが別のインスタンスをインストールしないようにするだけです。

{{< /tab >}}

{{< /tabs >}}

### ドメインネームサービスを設定する {#configure-domain-name-services}

DNSレコードを追加できる、インターネットからアクセス可能なドメインが必要です。

ドメインをGitLabコンポーネントに接続する方法の詳細については、[ネットワーキングとDNSのドキュメント](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns)を参照してください。GitLabカスタムリソース（CR）を定義するときは、このセクションで説明されている設定を使用します。

OpenShiftのIngressでは、特別な考慮事項が必要です。詳細については、[OpenShift Ingressに関する注意事項](openshift_ingress.md)を参照してください。

## GitLab Operatorのインストール {#installing-the-gitlab-operator}

まず、インストール方法を選択します。

{{< tabs >}}

{{< tab title="マニフェスト" >}}

まず、[Operatorリリースページ](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)からリリースマニフェストを取得します。ターゲットプラットフォームに一致するマニフェストを選択します: KubernetesまたはOpenShift。

次に、Operatorがインストールされるネームスペースを作成します。マニフェストでは、ネームスペースはデフォルトで`gitlab-system`に設定されています。ネームスペースを変更するには、マニフェストを手動で更新するか、このキーなどを簡単に設定できるHelmチャートの使用を検討してください。

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

使用可能なすべての設定オプションについては、[`values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml)を参照してください。

{{< /tab >}}

{{< tab title="OLM" >}}

GitLab Operatorは、次のOLMチャネルで使用できます:

- [OperatorHub.io](https://operatorhub.io/operator/gitlab-operator-kubernetes)
- [OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod)（OpenShiftおよびOKDの埋め込みOperatorHub内）
- [Red Hat Ecosystem Catalog](https://catalog.redhat.com/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f)

{{< /tab >}}

{{< /tabs >}}

Operatorデプロイのステータスを確認して、インストールを確認します:

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

## GitLabのインストール {#installing-gitlab}

1. GitLabカスタムリソース（CR）を作成します。

   `mygitlab.yaml`のような名前の新しいファイルを作成します。

   このファイルに入れるコンテンツの例を次に示します:

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

   `spec.chart.values`で使用する設定オプションの詳細については、[GitLab Helmチャートのドキュメント](https://docs.gitlab.com/charts/charts/)を参照してください。

1. 新しいGitLab CRを使用して、GitLabインスタンスをデプロイします。

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   このコマンドは、GitLab CRをクラスターに送信して、GitLab Operatorを調整します。コントローラーポッドからのログをトラブルシューティングすることで、進行状況を監視できます:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   GitLabリソースを一覧表示して、ステータスを確認することもできます:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

  CRが調整されると（GitLabリソースのステータスが`Running`になります）、ブラウザで`https://gitlab.example.com`のGitLabにアクセスできます。

ログインするには、デプロイの初期ルートパスワードを取得する必要があります。詳細な手順については、[Helmチャートのドキュメント](https://docs.gitlab.com/charts/installation/deployment/#initial-login)を参照してください。

## 推奨される次のステップ {#recommended-next-steps}

インストールが完了したら、[推奨される次のステップ](https://docs.gitlab.com/install/next_steps/)（認証オプションやサインアップ制限など）を実行することを検討してください。

### OpenShift {#openshift}

OpenShiftを実行する場合は、GitLab Operatorの承認戦略を自動（デフォルト）から手動に変更します。これにより、[承認が得られる](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators)まで、OpenShiftが新しいOperatorバージョンをインストールできなくなります。

Operatorのバージョンを固定するか、最新以外のバージョンにアップグレードするために、カスタム[`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster)を設定することもできます。

- 承認戦略は、[OpenShift Webコンソール](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf)から変更するか、[サブスクリプションの編集](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm)によって変更できます。
- `InstallPlan`の`.spec.approved`を`true`に設定して、手動アップグレードを承認します。
- 各GitLab Operatorは、定義されたGitLabチャートバージョンのサブセットをサポートしています: Operatorのアップグレードには、GitLabカスタムリソースのチャートバージョンの更新も含まれている必要があります。
- GitLab Operatorと指定されたGitLab Helmチャートバージョンに互換性がない場合、チャートへの設定変更は失敗し、[GitLab Helmチャートバージョンに関するエラー](gitlab_upgrades.md)が発生する可能性があります。

{{< alert type="note" >}}

[OLMはOperatorのダウングレードをサポートしていません](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177)。

{{< /alert >}}

## GitLab Operatorのアンインストール {#uninstall-the-gitlab-operator}

GitLab Operatorとその関連リソースを削除するには、以下の手順に従ってください。

Operatorをアンインストールする前に注意すべき事項:

- Operatorは、GitLabインスタンスが削除されても、永続ボリュームクレームまたはシークレットを削除しません。
- Operatorを削除すると、インストールされているネームスペース（デフォルトでは`gitlab-system`）は自動的に削除されません。これにより、永続ボリュームが誤って失われることがなくなります。

### GitLabのインスタンスのアンインストール {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

これにより、GitLabインスタンス、および上記の永続ボリュームクレームを除くすべての関連オブジェクトが削除されます）。

### GitLab Operatorのアンインストール {#uninstall-the-gitlab-operator-1}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

これにより、Operatorの実行中のデプロイを含む、Operatorのリソースが削除されます。これは、GitLabインスタンスに関連付けられたオブジェクトを**does not**。

## GitLab Operatorのトラブルシューティング {#troubleshoot-the-gitlab-operator}

Operatorのトラブルシューティングについては、[troubleshooting.md](troubleshooting.md)を参照してください。
