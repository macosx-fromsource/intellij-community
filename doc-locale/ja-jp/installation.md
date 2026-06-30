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

> [!note]
> GitLab Operatorには[既知の制限事項](_index.md#known-issues)があり、本番環境での使用は特定のシナリオにのみ適しています。

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->

GitLab Operatorには、以下の外部インスタンスが必要です。

- [PostgreSQL](https://docs.gitlab.com/charts/advanced/external-db/)
- [Redis](https://docs.gitlab.com/charts/advanced/external-redis/)
- [オブジェクトストレージ](https://docs.gitlab.com/charts/advanced/external-object-storage/)

本番環境のデプロイでは、[クラウドネイティブリファレンスアーキテクチャ](https://docs.gitlab.com/administration/reference_architectures)に従ってください。

このドキュメントでは、KubernetesまたはOpenShiftクラスターでマニフェストを使用してGitLab Operatorをデプロイする方法について説明します。

<!--This warning block is duplicated in `../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml`.
Changes should be reflected in both locations.-->

OpenShiftを使用する場合、インストールは通常Operator Lifecycle Manager（OLM）によって処理されます。
**OLMを使用したインストールは実験的とみなされます**。GitLabは、OLMを使用してデプロイされたインスタンスに関連する問題をサポートしていません。
OLMの潜在的な問題の詳細については、[イシュー241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241)を参照してください。

## 前提条件 {#prerequisites}

1. [Kubernetesまたは既存のOpenShiftクラスターを作成または使用する](#cluster)
1. 前提条件となるサービスとソフトウェアをインストールする
   - [Ingress/Gateway APIコントローラー](#external-traffic-routing)
   - [cert-manager](#tls-certificates)
   - [メトリクスサーバー](#metrics)
1. [ドメイン名サービスを設定する](#configure-domain-name-services)

### クラスター {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

従来のKubernetesクラスターを作成するには、[公式ツール](https://kubernetes.io/docs/tasks/tools/)またはお好みのインストール方法を使用することを検討してください。

GitLab Operatorは以下のKubernetesバージョンをサポートしています:

| Kubernetesリリース | ステータス | 最小Operatorバージョン |
|--------------------|-------------|--------------------------|
| 1.35               | サポート済み   | 2.9.0                    |
| 1.34               | サポート済み   | 2.5.0                    |
| 1.33               | サポート済み   | 2.1.0                    |
| 1.32               | 非推奨  | 2.0.0                    |
| 1.31               | サポート対象外 | 1.9.0                    |

{{< /tab >}}

{{< tab title="OpenShift" >}}

GitLab Operatorは以下のOpenShiftバージョンをサポートしています:

| OpenShiftリリース | ステータス | 最小Operatorバージョン |
|-------------------|-------------|--------------------------|
| 4.21              | サポート済み   | 2.9.0                    |
| 4.20              | サポート済み   | 2.6.0                    |
| 4.19              | サポート済み   | 2.2.0                    |
| 4.18              | サポート済み   | 1.9.0                    |
| 4.17              | サポート対象外 | 1.6.0                    |

{{< /tab >}}

{{< /tabs >}}

Kubernetesの直近3つのマイナーバージョンと、OpenShiftの直近4つのマイナーリリースとの互換性を同時にターゲットとしています。新しいバージョンのサポートが追加されると、最も古いサポート対象バージョンのテストは終了します。KubernetesおよびOpenShiftの新しいマイナーリリースに対して、初回リリースから3か月以内にOperatorサポートを提供することを目標としています。

詳細については、[Kubernetesサポートポリシーを参照してください](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)。

> [!note]
> [Kubernetes用エージェント](https://docs.gitlab.com/user/clusters/agent/)や[GitLabチャート](https://docs.gitlab.com/charts/installation/cloud/)など一部のコンポーネントでは、GitLabが異なるクラスターバージョンをサポートする場合があります。

上記に記載されているバージョンより新しいリリースとの互換性の問題については、[イシュートラッカー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)でお知らせください。

非推奨バージョンおよび上記に記載されているバージョンより古いバージョンでは、一部のGitLab機能が動作しない場合があります。

Operatorはx86-64とARM64をサポートしています。ARM64ビルドは16.7から利用可能でしたが、完全なサポートとテストカバレッジは18.8から提供されています。

### 外部トラフィックルーティング {#external-traffic-routing}

アプリケーションへのアクセスを提供するには、外部トラフィックルーティングが必要です。
[Envoy Gateway](https://gateway.envoyproxy.io/)を使用した[Gateway API](https://gateway-api.sigs.k8s.io/)は、新規デプロイに推奨されるアプローチです。設定の詳細と代替プロバイダーについては、[Gateway APIドキュメント](gatewayapi.md)を参照してください。

バンドルされているNGINX IngressコントローラーはGitLabチャート19.0で非推奨となり、GitLab 20.0で削除される予定です。代替として[外部NGINX Ingressコントローラー](https://docs.gitlab.com/charts/advanced/external-ingress/)を使用できます。

### TLS証明書 {#tls-certificates}

OperatorのKubernetes Webhookの証明書を作成するために、[cert-manager](https://cert-manager.io)が使用されます。GitLabの証明書にも[cert-manager](https://cert-manager.io)を使用することをお勧めします。

OperatorはKubernetes Webhookの証明書を必要とするため、GitLabチャートにバンドルされているcert-managerは使用できません。代わりに、Operatorをインストールする前にcert-managerをインストールしてください。

[インストールドキュメント](https://cert-manager.io/docs/installation/)に従って、お使いのプラットフォームとツールに対応した[サポート対象のcert-managerリリース](https://cert-manager.io/docs/releases/)をインストールしてください。

### メトリクス {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

HorizontalPodAutoscalerがポッドのメトリクスを取得できるように、[メトリクスサーバー](https://github.com/kubernetes-sigs/metrics-server#installation)をインストールしてください。

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShiftにはデフォルトで[Prometheus Adapter](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html)が搭載されているため、GitLab Operatorが別のインスタンスをインストールしないよう、GitLabカスタムリソースで`spec.chart.values.prometheus.install=false`を設定するだけで済みます。

{{< /tab >}}

{{< /tabs >}}

### ドメイン名サービスの設定 {#configure-domain-name-services}

DNSレコードを追加できる、インターネットからアクセス可能なドメインが必要です。

ドメインをGitLabコンポーネントに接続する方法の詳細については、[ネットワークとDNSのドキュメント](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns)を参照してください。このセクションで説明する設定は、GitLabカスタムリソース（CR）を定義する際に使用します。

OpenShiftのIngressには追加の考慮事項があります。詳細については、[OpenShift Ingressに関する注意事項](openshift_ingress.md)を参照してください。

## GitLab Operatorのインストール {#installing-the-gitlab-operator}

まず、インストール方法を選択してください。

{{< tabs >}}

{{< tab title="Manifest" >}}

まず、[Operatorリリースページ](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)からリリースマニフェストを取得します。各リリースには4つのマニフェストが公開されています。ターゲットプラットフォームと必要なRBACスコープに合ったものを選択してください:

| マニフェスト                                       | プラットフォーム   | RBACスコープ                                                                                            |
|------------------------------------------------|------------|-------------------------------------------------------------------------------------------------------|
| `gitlab-operator-kubernetes.yaml`              | Kubernetes | クラスター全体: `ClusterRole`/`ClusterRoleBinding`。OperatorはすべてのネームスペースのGitLab CRを監視します。  |
| `gitlab-operator-kubernetes-namespaced.yaml`   | Kubernetes | ネームスペース限定: インストールネームスペースにスコープされた`Role`/`RoleBinding`。Operatorはそのネームスペースのみを監視します。クラスタースコープのリソースには小さな`ClusterRole`が引き続き必要です。 |
| `gitlab-operator-openshift.yaml`               | OpenShift  | クラスター全体（上記と同様）。                                                                              |
| `gitlab-operator-openshift-namespaced.yaml`    | OpenShift  | ネームスペース限定（上記と同様）。                                                                                |

単一のOperatorで複数のネームスペースにまたがるGitLabインスタンスを管理する場合は、クラスター全体のバリアントを使用してください。ターゲットクラスターでクラスター全体のRBACの付与が許可されていないなど、Operatorを単一のネームスペースに限定する必要がある場合は、ネームスペース限定のバリアントを使用してください。ネームスペース限定のバリアントでは、GitLabカスタムリソースはOperatorと同じネームスペースに作成する必要があります。

次に、Operatorをインストールするネームスペースを作成します。マニフェストでは、ネームスペースはデフォルトで`gitlab-system`に設定されています。ネームスペースを変更するには、マニフェストを手動で更新するか、このキーやその他のキーを簡単に設定できるHelmチャートの使用を検討してください。

```shell
kubectl create namespace gitlab-system
```

最後に、マニフェストを適用します:

```shell
kubectl apply -f gitlab-operator-<platform>.yaml
```

{{< /tab >}}

{{< tab title="Helm Chart" >}}

まず、GitLab Helmリポジトリを追加して最新の更新を取得します。

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

GitLab Operatorは以下のOLMチャンネルで利用可能です:

- [OperatorHub.io](https://operatorhub.io/operator/gitlab-operator-kubernetes)
- [OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod)（OpenShiftおよびOKDに組み込まれたOperatorHub内）
- [Red Hat Ecosystem Catalog](https://catalog.redhat.com/en/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f)

{{< /tab >}}

{{< /tabs >}}

Operator Deploymentのステータスを確認してインストールを確認します:

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

### ネームスペース限定モードでのバンドルNGINX IngressとPrometheus {#bundled-nginx-ingress-and-prometheus-in-namespaced-mode}

ネームスペース限定のマニフェストバリアント（および`watchCluster=false`でのHelmインストール）では、チャートにバンドルされているNGINX IngressコントローラーとPrometheusサーバーの`ClusterRole`と`ClusterRoleBinding`が省略されます。これら2つのコンポーネントはクラスター全体で動作するように設計されています:

- NGINX Ingressはすべてのネームスペースの`Ingresses`を監視し、`nodes`や`ingressclasses`などのクラスタースコープのリソースを読み取ります。
- Prometheusサーバーはクラスター全体のターゲットを検出してスクレイピングし、`nodes`、`nodes/proxy`、`nodes/metrics`、および`/metrics`非リソースURLへのアクセスが必要です。

ネームスペース限定モードでOperatorをインストールし、これらのバンドルコンポーネントを使用する場合は、クラスタースコープの権限を自分で提供する必要があります（通常、チャートの`gitlab-nginx-ingress`/`gitlab-prometheus-server` `ServiceAccount`を外部管理の`ClusterRole`にバインドし、NGINX Ingressの場合はコントローラーに`--watch-namespace`を渡すことで対応します）。

ネームスペース限定モードで推奨されるアプローチは、外部管理のモニタリングおよびIngress/Gateway APIソリューションを使用することです。

## GitLabのインストール {#installing-gitlab}

1. GitLabカスタムリソース（CR）を作成します。

   `mygitlab.yaml`のような名前の新しいファイルを作成します。

   このファイルに記述する内容の例を以下に示します:

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

   `spec.chart.values`以下で使用する設定オプションの詳細については、[GitLab Helmチャートドキュメント](https://docs.gitlab.com/charts/charts/)を参照してください。

1. 新しいGitLab CRを使用してGitLabインスタンスをデプロイします。

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   このコマンドにより、GitLab CRがクラスターに送信され、GitLab Operatorによって調整されます。コントローラーポッドのログをテールすることで進捗を確認できます:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   GitLabリソースを一覧表示してステータスを確認することもできます:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

   CRが調整されると（GitLabリソースのステータスが`Running`になると）、ブラウザで`https://gitlab.example.com`からGitLabにアクセスできます。

ログインするには、デプロイメントの初期rootパスワードを取得する必要があります。詳細な手順については、[Helmチャートドキュメント](https://docs.gitlab.com/charts/installation/deployment/#initial-login)を参照してください。

## 推奨される次のステップ {#recommended-next-steps}

インストールが完了したら、認証オプションやサインアップ制限など、[推奨される次のステップ](https://docs.gitlab.com/install/next_steps/)を実施することを検討してください。

### OpenShift {#openshift}

OpenShiftを使用している場合は、GitLab Operatorの承認ストラテジーを自動（デフォルト）から手動に変更してください。これにより、[承認が与えられる](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators)まで、OpenShiftが新しいOperatorバージョンをインストールしないようになります。

カスタム[`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster)を設定して、Operatorのバージョンを固定したり、最新でないバージョンにアップグレードしたりすることもできます。

- 承認ストラテジーは、[OpenShift Webコンソール](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf)から変更するか、[Subscriptionを編集する](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm)ことで変更できます。
- `InstallPlan`の`.spec.approved`を`true`に設定して、手動アップグレードを承認します。
- 各GitLab Operatorは定義されたGitLabチャートバージョンのサブセットをサポートしています。GitLab Operatorへのアップグレードには、GitLabカスタムリソースのチャートバージョンの更新も必要です。
- GitLab Operatorと指定されたGitLab HelmチャートバージョンにGitLab Helmチャートバージョンに関するエラーが発生してチャートへの設定変更が失敗する場合があります。詳細については、[GitLab Helmチャートバージョンに関するエラー](gitlab_upgrades.md)を参照してください。

> [!note]
> [OLMはOperatorのダウングレードをサポートしていません](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177)。

## GitLab Operatorのアンインストール {#uninstall-the-gitlab-operator}

GitLab Operatorとその関連リソースを削除するには、以下の手順に従ってください。

Operatorをアンインストールする前に注意すべき事項:

- GitLabインスタンスが削除されても、OperatorはPersistent Volume ClaimsやSecretsを削除しません。
- Operatorを削除する際、インストールされているネームスペース（デフォルトでは`gitlab-system`）は自動的に削除されません。これにより、永続ボリュームが意図せず失われることを防ぎます。

### GitLabインスタンスのアンインストール {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

これにより、GitLabインスタンスと、上記で述べたPersistent Volume Claimsを除くすべての関連オブジェクトが削除されます。

### GitLab Operatorのアンインストール {#uninstall-the-gitlab-operator}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

これにより、Operatorの実行中のDeploymentを含むOperatorのリソースが削除されます。GitLabインスタンスに関連するオブジェクトは**削除されません**。

## GitLab Operatorのトラブルシューティング {#troubleshoot-the-gitlab-operator}

GitLab Operatorのトラブルシューティングについては、[トラブルシューティング](troubleshooting.md)を参照してください。
