---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: インストール
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 提供:GitLab Self-Managed

{{< /details >}}

{{< alert type="note" >}}

GitLab Operatorには[既知の制限事項](_index.md#known-issues)があり、本番環境での特定のシナリオにのみ適しています。

{{< /alert >}}

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->
{{< alert type="warning" >}}

_GitLabカスタムリソース_のデフォルト値は、**本番環境での使用を想定していません**。これらの値を使用すると、GitLab Operatorは、永続データを含む_すべての_サービスがKubernetesクラスターにデプロイされるGitLabインスタンスを作成しますが、これは**本番ワークロードには適していません**。本番環境へのデプロイメントでは、**必ず**[クラウドネイティブハイブリッド参照アーキテクチャ](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)に従ってください。GitLabは、Kubernetesクラスター内にデプロイされたPostgreSQL、Redis、Gitaly、Praefect、またはMinIOに関連する問題はサポートしません。{{< /alert >}}

このドキュメントでは、KubernetesまたはOpenShiftクラスターでマニフェストを使用してGitLab Operatorをデプロイする方法について説明します。

<!--This warning block is duplicated in ../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml.
Changes should be reflected in both locations.-->
OpenShiftを使用している場合、通常、インストールはOperator Lifecycle Manager (OLM) によって処理されます。**OLMを使用したインストールは実験的であると見なされます**。GitLabは、OLMを使用してデプロイされたインスタンスに関連する問題はサポートしません。OLMの潜在的な問題に関する詳細については、[イシュー241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241)を参照してください。

## 前提要件 {#prerequisites}

1. [既存のKubernetesまたはOpenShiftクラスターを作成または使用する](#cluster)
1. 前提条件となるサービスとソフトウェアをインストールする
   - [Ingressコントローラー](#ingress-controller)
   - [cert-manager](#tls-certificates)
   - [メトリクスサーバー](#metrics)
1. [ドメインネームサービスの設定](#configure-domain-name-services)

### クラスター {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

従来のKubernetesクラスターを作成するには、[公式ツール](https://kubernetes.io/docs/tasks/tools/)または推奨のインストール方法を使用することを検討してください。

GitLab Operatorは、次のKubernetesバージョンをサポートしています。

| Kubernetesリリース | 状態      | 最小Operatorバージョン | アーキテクチャ | サポート終了 |
|--------------------|-------------|--------------------------|---------------|-------------|
| 1.33               | サポート対象   | 2.1.0                    | x86-64        | 2026-06-28  |
| 1.32               | サポート対象   | 2.0.0                    | x86-64        | 2026-02-28  |
| 1.31               | サポート対象   | 1.9.0                    | x86-64        | 2025-10-28  |
| 1.30               | 非推奨  | 1.6.0                    | x86-64        | 2025-06-28  |
| 1.29               | サポート対象外 | 1.0.0                    | x86-64        | 2025-02-28  |
| 1.28               | サポート対象外 | 1.0.0                    | x86-64        | 2024-10-28  |
| 1.27               | サポート対象外 | 0.29.0                   | x86-64        | 2024-06-28  |
| 1.26               | サポート対象外 | 0.24.0                   | x86-64        | 2024-02-28  |
| 1.25               | サポート対象外 | 0.24.0                   | x86-64        | 2023-10-28  |
| 1.24               | サポート対象外 | 0.24.0                   | x86-64        | 2023-07-28  |
| 1.23               | サポート対象外 | 0.24.0                   | x86-64        | 2023-02-28  |
| 1.22               | サポート対象外 | 0.24.0                   | x86-64        | 2022-10-28  |

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShiftクラスターを作成するには、[OpenShiftクラスターのセットアップに関するドキュメント](developer/openshift_cluster_setup.md)で、_開発環境_を作成する方法の例を参照してください。

GitLab Operatorは、次のOpenShiftバージョンをサポートしています。

| OpenShiftリリース | 状態    | 最小Operatorバージョン | アーキテクチャ | サポート終了 |
|-------------------|-----------|--------------------------|---------------|-------------|
| 4.18              | サポート対象 | 1.9.0                    | x86-64        | 2028-02-25  |
| 4.17              | サポート対象 | 1.6.0                    | x86-64        | 2026-04-01  |
| 4.16              | サポート対象 | 1.3.0                    | x86-64        | 2027-06-27  |
| 4.15              | サポート対象 | 0.31.0                   | x86-64        | 2025-08-27  |
| 4.14              | サポート対象 | 0.27.0                   | x86-64        | 2026-10-31  |
| 4.13              | サポート対象 | 0.24.0                   | x86-64        | 2024-11-17  |
| 4.12              | サポート対象 | 0.24.0                   | x86-64        | 2026-01-17  |

{{< /tab >}}

{{< /tabs >}}

GitLab Operatorは、新しいマイナーバージョンのKubernetesおよびOpenShiftバージョンを、最初のリリースから3か月後にサポートすることを目指しています。上記のリストよりも新しいリリースに関する互換性の問題は、[イシュートラッカー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)でお知らせください。

一部のGitLab機能は、非推奨バージョン、および上記のバージョンよりも古いバージョンでは動作しない場合があります。

[Kubernetes用エージェント](https://docs.gitlab.com/user/clusters/agent/)や[GitLabチャート](https://docs.gitlab.com/charts/installation/cloud/)など、一部のコンポーネントでは、GitLabが異なるクラスターバージョンをサポートしている場合があります。

16.7以降、Operatorはx86-64およびarm64向けにビルドされています。arm64イメージはCIでテストされておらず、本番環境での使用は推奨されません。

マルチアーキテクチャクラスターを使用している場合は、Operator Deploymentに[`kubernetes.io/arch`ラベル](https://kubernetes.io/docs/reference/node/node-labels/#preset-labels)の[ノードセレクター](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)を追加することをお勧めします。

x86-64/amd64ノードでのみスケジュールされるようにデプロイメントにパッチを適用します:

```shell
kubectl patch deployments gitlab-controller-manager \
  -p '{"spec": {"template": {"spec": {"nodeSelector": {"kubernetes.io/arch": "amd64"}}}}}'
```

Operator Helmチャートを使用している場合は、代わりに`values.yaml`にノードセレクターを追加できます:

```yaml
nodeSelector:
  kubernetes.io/arch: amd64
```

これにより、Operatorはテストするプラットフォームを使用して`amd64`ノードで実行されるようになります。

CNGイメージのarm64サポートの詳細については、[epic 10928](https://gitlab.com/groups/gitlab-org/-/epics/10938)を参照してください。

### Ingressコントローラー {#ingress-controller}

Ingressコントローラーは、アプリケーションへの外部アクセスを提供し、コンポーネント間の通信を保護するために必要です。

GitLab Operatorは、デフォルトで[GitLab HelmチャートからフォークしたNGINXチャート](https://docs.gitlab.com/charts/charts/nginx/)をデプロイします。

外部Ingressコントローラーを使用する場合は、Kubernetesコミュニティの[NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/)を使用してIngressコントローラーをデプロイします。プラットフォームと推奨ツールに基づいて、リンク内の関連する手順に従ってください。後で使用するためにIngressクラスの値に注意してください（通常、デフォルトは`nginx`です）。GitLab CRを構成する場合は、GitLab HelmチャートからNGINXオブジェクトを無効にするために、必ず`nginx-ingress.enabled=false`を設定してください。

### TLS証明書 {#tls-certificates}

OperatorのKubernetes Webhookの証明書を作成するには、[cert-manager](https://cert-manager.io)を使用します。GitLab証明書にも[cert-manager](https://cert-manager.io)を使用する必要があります。

OperatorにはKubernetes Webhookの証明書が必要なため、GitLabチャートにバンドルされているcert-managerは使用できません。代わりに、Operatorをインストールする前にcert-managerをインストールします。

[インストールに関するドキュメント](https://cert-manager.io/docs/installation/)に従って、プラットフォームとツール用の[サポートされているcert-managerリリース](https://cert-manager.io/docs/releases/)をインストールします。

### メトリクス {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

HorizontalPodAutoscalerがポッドメトリックを取得できるように、[メトリックサーバー](https://github.com/kubernetes-sigs/metrics-server#installation)をインストールします。

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShiftにはデフォルトで[Prometheusアダプター](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html)が付属しているため、GitLab Operatorが別のインスタンスをインストールしないようにするには、GitLabカスタムリソースで`spec.chart.values.prometheus.install=false`を設定するだけです。

{{< /tab >}}

{{< /tabs >}}

### ドメインネームサービスの設定 {#configure-domain-name-services}

DNSレコードを追加できる、インターネットからアクセス可能なドメインが必要です。

ドメインをGitLabコンポーネントに接続する方法の詳細については、[ネットワーキングおよびDNSドキュメント](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns)を参照してください。GitLabカスタムリソース (CR) を定義する際に、このセクションで説明されている構成を使用します。

OpenShiftのIngressには、特別な考慮事項が必要です。詳細については、[OpenShift Ingressに関するノート](openshift_ingress.md)を参照してください。

## GitLab Operatorのインストール {#installing-the-gitlab-operator}

まず、インストール方法を選択します。

{{< tabs >}}

{{< tab title="マニフェスト" >}}

まず、[Operatorリリースページ](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)からリリース マニフェストを取得します。ターゲットプラットフォームに一致するマニフェストを選択します:KuberentesまたはOpenShift。

次に、Operatorがインストールされるネームスペースを作成します。マニフェストでは、ネームスペースはデフォルトで`gitlab-system`に設定されています。ネームスペースを変更するには、マニフェストを手動で更新するか、このキーなどを簡単に構成できるHelmチャートを使用することを検討してください。

```shell
kubectl create namespace gitlab-system
```

最後に、マニフェストを適用します:

```shell
kubectl apply -f gitlab-operator-<platform>.yaml
```

{{< /tab >}}

{{< tab title="Helm Chart" >}}

まず、GitLab Helmリポジトリを追加し、最新の更新を取得します。

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

使用可能なすべての構成オプションについては、[`values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml)を参照してください。

{{< /tab >}}

{{< tab title="OLM" >}}

GitLab Operatorは、次のOLMチャンネルで利用できます。

| チャンネル                                                                                                 | リスティング |
|---------------------------------------------------------------------------------------------------------|---------|
| [OperatorHub Community Operators](https://github.com/k8s-operatorhub/community-operators)               | [リンク](https://operatorhub.io/operator/gitlab-operator-kubernetes) |
| [OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod) | OpenShiftおよびOKDの埋め込みOperatorHubで利用可能 |
| [OpenShift Certified Operators](https://github.com/redhat-openshift-ecosystem/certified-operators)      | [リンク](https://catalog.redhat.com/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f) |

{{< /tab >}}

{{< /tabs >}}

Operator Deploymentの状態を確認して、インストールを確認します:

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

## GitLabのインストール {#installing-gitlab}

1. GitLabカスタムリソース (CR) を作成します。

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

   `spec.chart.values`で使用する構成オプションの詳細については、[GitLab Helmチャートのドキュメント](https://docs.gitlab.com/charts/charts/)を参照してください。

1. 新しいGitLab CRを使用してGitLabインスタンスをデプロイします。

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   このコマンドは、GitLab Operatorが調整するために、GitLab CRをクラスターに送信します。コントローラーポッドからのlogを末尾に追加することで、進行状況を監視できます:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   GitLabリソースをリストして、その状態を確認することもできます:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

  CRが調整されると (GitLabリソースの状態が`Running`になると)、ブラウザーで`https://gitlab.example.com`のGitLabにアクセスできます。

ログインするには、デプロイメントの初期rootパスワードを取得する必要があります。詳細な手順については、[Helmチャートのドキュメント](https://docs.gitlab.com/charts/installation/deployment/#initial-login)を参照してください。

## 推奨される次のステップ {#recommended-next-steps}

インストールが完了したら、認証オプションやサインアップ制限など、[推奨される次のステップ](https://docs.gitlab.com/install/next_steps/)を実行することを検討してください。

### OpenShift {#openshift}

OpenShiftを実行する場合は、GitLab Operatorの承認ストラテジーを自動 (デフォルト) から手動に変更します。これにより、[承認が得られる](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators)まで、OpenShiftが新しいOperatorバージョンをインストールできなくなります。

カスタム[`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster)を設定して、Operatorのバージョンをピン留めしたり、最新バージョン以外のバージョンにアップグレードしたりすることもできます。

- 承認ストラテジーは、[OpenShift Webコンソール](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf)または[サブスクリプションの編集](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm)によって変更できます。
- `.spec.approved`を`true`の`InstallPlan`に設定して、手動アップグレードを承認します。
- 各GitLab Operatorは、定義されたGitLabチャートバージョンのサブセットをサポートしています。GitLab Operatorへのアップグレードでは、GitLabカスタムリソースのチャートバージョンも更新する必要があります。
- Operatorと指定されたチャートバージョンに互換性がない場合、チャートの構成変更は[チャートバージョンに関するエラー](operator_upgrades.md#step-5-update-the-chart-version-in-the-gitlab-custom-resource-cr)で失敗する可能性があります。

{{< alert type="note" >}}

[OLMは現在、Operatorのダウングレードをサポートしていません](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177)。

{{< /alert >}}

## GitLab Operatorのアンインストール {#uninstall-the-gitlab-operator}

GitLab Operatorとそれに関連付けられたリソースを削除するには、次の手順に従ってください。

Operatorをアンインストールする前に注意すべき項目:

- Operatorは、GitLabインスタンスが削除されても、永続ボリュームクレームまたはシークレットを削除しません。
- Operatorを削除すると、インストール先のネームスペース (デフォルトでは`gitlab-system`) は自動的に削除されません。これにより、永続ボリュームが誤って失われることがなくなります。

### GitLabのインスタンスのアンインストール {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

これにより、GitLabインスタンス、および上記の永続ボリュームクレームを除く、関連するすべてのオブジェクトが削除されます）。

### GitLab Operatorのアンインストール {#uninstall-the-gitlab-operator-1}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

これにより、Operatorの実行中のDeploymentを含む、Operatorのリソースが削除されます。これはGitLab**インスタンス**に関連付けられたオブジェクトを削除**しません**。

## GitLab Operator {#troubleshoot-the-gitlab-operator}のトラブルシューティング

Operatorの[トラブルシューティング](troubleshooting.md)については、[troubleshooting.md](troubleshooting.md)を参照してください。
