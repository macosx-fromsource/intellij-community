---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: インストール
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

{{< alert type="note" >}}

GitLabオペレーターには[既知の制限事項](_index.md#known-issues)があり、本番環境での使用における特定のシナリオにのみ適しています。

{{< /alert >}}

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->

{{< alert type="warning" >}}

GitLabカスタムリソースのデフォルト値は、**本番環境での使用を意図していません**。これらの値を使用すると、GitLabオペレーターは、永続データを含むすべてのサービスがKubernetesクラスターにデプロイされるGitLabインスタンスを作成しますが、これは**本番環境のワークロードに適していません**。本番環境へのデプロイでは、[クラウドネイティブハイブリッドリファレンスアーキテクチャ](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)に従う**必要があります**。GitLabは、Kubernetesクラスター内にデプロイされたPostgreSQL、Redis、Gitaly、Praefect、またはMinIOに関連するイシューをサポートしません。

{{< /alert >}}

このドキュメントでは、KubernetesまたはOpenShiftクラスターでマニフェストを使用してGitLabオペレーターをデプロイする方法について説明します。

<!--This warning block is duplicated in ../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml.
Changes should be reflected in both locations.-->
OpenShiftを使用している場合、インストールは通常、Operator Lifecycle Manager（OLM）によって処理されます。**OLMを使用したインストールは、試験的であると見なされます**。GitLabは、OLMを使用してデプロイされたインスタンスに関連するイシューをサポートしません。OLMの潜在的なイシューに関する詳細については、[イシュー241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241)を参照してください。

## 前提要件 {#prerequisites}

1. [既存のKubernetesまたはOpenShiftクラスターを作成または使用します](#cluster)
1. 前提条件となるサービスとソフトウェアをインストールします
   - [Ingressコントローラー](#ingress-controller)
   - [cert-manager](#tls-certificates)
   - [メトリクスサーバー](#metrics)
1. [ドメインネームシステムを設定する](#configure-domain-name-services)

### クラスタ: {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

従来のKubernetesクラスターを作成するには、[公式ツール](https://kubernetes.io/docs/tasks/tools/)または推奨インストール方法の使用を検討してください。

GitLabオペレーターは、次のKubernetesバージョンをサポートしています:

| Kubernetesリリース | ステータス      | 最小オペレーターバージョン | アーキテクチャ |
|--------------------|-------------|--------------------------|---------------|
| 1.34               | サポート対象   | 2.6.0                    | x86-64        |
| 1.33               | サポート対象   | 2.1.0                    | x86-64        |
| 1.32               | サポート対象   | 2.0.0                    | x86-64        |
| 1.31               | 非推奨  | 1.9.0                    | x86-64        |
| 1.30               | サポート対象外 | 1.6.0                    | x86-64        |

{{< /tab >}}

{{< tab title="OpenShift" >}}

GitLabオペレーターは、次のOpenShiftバージョンをサポートしています:

| OpenShiftリリース | ステータス      | 最小オペレーターバージョン | アーキテクチャ |
|-------------------|-------------|--------------------------|---------------|
| 4.19              | サポート対象   | 2.2.0                    | x86-64        |
| 4.18              | サポート対象   | 1.9.0                    | x86-64        |
| 4.17              | サポート対象   | 1.6.0                    | x86-64        |
| 4.16              | サポート対象   | 1.3.0                    | x86-64        |
| 4.15              | サポート対象外 | 0.31.0                   | x86-64        |

{{< /tab >}}

{{< /tabs >}}

当社は、Kubernetesの3つの最新のマイナーバージョンと、OpenShiftの4つの最新のマイナーリリースとの互換性を同時に目指しています。新しいバージョンのサポートが追加されると、最も古いサポート対象バージョンのテストは中止されます。私たちの目標は、KubernetesとOpenShiftの新しいマイナーリリースの最初の提供から3か月以内に、オペレーターのサポートを提供することです。

詳細については、[Kubernetesサポートポリシーを参照してください](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)。

ノート: 一部のコンポーネント（[Kubernetes用のエージェント](https://docs.gitlab.com/user/clusters/agent/)や[GitLab Charts](https://docs.gitlab.com/charts/installation/cloud/)など）では、GitLabが異なるクラスターのバージョンをサポートする場合があります。

上記のリストよりも新しいリリースとの互換性に関するイシューは、[イシュートラッカー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)でお知らせください。

一部のGitLab機能は、非推奨のバージョンおよび上記のバージョンより古いバージョンでは機能しない場合があります。

16.7以降、オペレーターはx86-64およびarm64用に構築されています。arm64イメージは継続的インテグレーションでテストされておらず、本番環境での使用は推奨されません。

マルチアーキテクチャクラスターを使用している場合は、[ノードセレクター](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector)を[`kubernetes.io/arch`ラベル](https://kubernetes.io/docs/reference/node/node-labels/#preset-labels)をオペレーターデプロイに追加することをお勧めします。

デプロイにパッチを適用して、x86-64/amd64ノードでのみスケジュールされるようにします:

```shell
kubectl patch deployments gitlab-controller-manager \
  -p '{"spec": {"template": {"spec": {"nodeSelector": {"kubernetes.io/arch": "amd64"}}}}}'
```

オペレーターHelmチャートを使用している場合は、`values.yaml`にノードセレクターを追加できます:

```yaml
nodeSelector:
  kubernetes.io/arch: amd64
```

これにより、テストするプラットフォームを使用して、オペレーターが`amd64`ノードで実行されるようになります。

CNGイメージのarm64サポートの詳細については、[エピック10928](https://gitlab.com/groups/gitlab-org/-/epics/10938)を参照してください。

### Ingressコントローラー {#ingress-controller}

Ingressコントローラーは、アプリケーションへの外部アクセスを提供し、コンポーネント間のセキュアな通信を確保するために必要です。

GitLabオペレーターはデフォルトで[フォークしたNGINXチャートをGitLab Helmチャート](https://docs.gitlab.com/charts/charts/nginx/)からデプロイします。

外部のIngressコントローラーを使用する場合は、Kubernetesコミュニティの[NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/)を使用してIngressコントローラーをデプロイします。プラットフォームと推奨ツールに基づいて、リンク内の関連する手順に従ってください。後で使用するために、Ingressクラスの値（通常は`nginx`がデフォルト）をメモしておきます。GitLab CRを設定するときは、GitLab HelmチャートからNGINXオブジェクトを無効にするために、必ず`nginx-ingress.enabled=false`を設定してください。

### TLS証明書 {#tls-certificates}

オペレーターのKubernetes Webhookの証明書を作成するには、[cert-manager](https://cert-manager.io)を使用します。GitLab証明書にも[cert-manager](https://cert-manager.io)を使用する必要があります。

オペレーターにはKubernetes Webhookの証明書が必要なため、GitLabチャートにバンドルされているcert-managerは使用できません。代わりに、オペレーターをインストールする前に、cert-managerをインストールします。

プラットフォームとツールに対応した[サポートされているcert-managerリリース](https://cert-manager.io/docs/releases/)をインストールするには、[インストールドキュメント](https://cert-manager.io/docs/installation/)に従ってください。

### メトリクス {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

HorizontalPodAutoscalersがポッドメトリクスを取得するできるように、[メトリクスサーバー](https://github.com/kubernetes-sigs/metrics-server#installation)をインストールします。

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShiftには[Prometheusアダプター](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html)がデフォルトで付属しているため、GitLabカスタムリソースで`spec.chart.values.prometheus.install=false`を設定するだけで、GitLabオペレーターが別のインスタンスをインストールするのを防ぐことができます。

{{< /tab >}}

{{< /tabs >}}

### ドメインネームシステムの設定 {#configure-domain-name-services}

DNSレコードを追加できる、インターネットからアクセス可能なドメインが必要です。

ドメインをGitLabコンポーネントに接続する方法の詳細については、[ネットワーキング](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns)とDNSに関するドキュメントを参照してください。GitLabカスタムリソース（CR）を定義するときは、このセクションで説明されている設定を使用します。

OpenShiftのIngressには、特別な考慮事項が必要です。詳細については、[OpenShift Ingressに関する注意事項](openshift_ingress.md)を参照してください。

## GitLabオペレーターのインストール {#installing-the-gitlab-operator}

まず、インストール方法を選択します。

{{< tabs >}}

{{< tab title="マニフェスト" >}}

まず、[オペレーターリリースページ](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)からリリースマニフェストを取得するします。ターゲットプラットフォームに一致するマニフェストを選択します: KubernetesまたはOpenShift。

次に、オペレーターがインストールされるネームスペースを作成します。マニフェストでは、ネームスペースはデフォルトで`gitlab-system`に設定されています。ネームスペースを変更するには、マニフェストを手動で更新するか、このキーなどを簡単に設定できるHelmチャートの使用を検討してください。

```shell
kubectl create namespace gitlab-system
```

最後に、マニフェストを適用します:

```shell
kubectl apply -f gitlab-operator-<platform>.yaml
```

{{< /tab >}}

{{< tab title="Helmチャート" >}}

まず、GitLab Helmリポジトリを追加し、最新の更新を取得するします。

```shell
helm repo add gitlab https://charts.gitlab.io
helm repo update
```

次に、GitLabオペレーターチャートをインストールできます:

```shell
helm install gitlab-operator gitlab/gitlab-operator \
  --create-namespace \
  --namespace gitlab-system
```

使用可能なすべての設定オプションについては、[`values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml)を参照してください。

{{< /tab >}}

{{< tab title="OLM" >}}

GitLabオペレーターは、次のOLMチャンネルで使用できます:

- [OperatorHub.io](https://operatorhub.io/operator/gitlab-operator-kubernetes)
- [OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod)（OpenShiftおよびOKDの埋め込みOperatorHub内）
- [Red Hat Ecosystem Catalog](https://catalog.redhat.com/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f)

{{< /tab >}}

{{< /tabs >}}

オペレーターデプロイのステータスを確認して、インストールを確認します:

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

## GitLabのインストール {#installing-gitlab}

1. GitLabカスタムリソース（CR）を作成します。

   `mygitlab.yaml`のような名前の新しいファイルを作成します。

   このファイルに含めるコンテンツの例を次に示します:

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

   `spec.chart.values`で使用する設定オプションの詳細については、[GitLab Helmチャートドキュメント](https://docs.gitlab.com/charts/charts/)を参照してください。

1. 新しいGitLab CRを使用してGitLabインスタンスをデプロイします。

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   このコマンドは、GitLab CRをクラスターに送信して、GitLabオペレーターが調整するようにします。コントローラーポッドからログを追跡することで、進行状況を監視できます:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   GitLabリソースを一覧表示して、ステータスを確認することもできます:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

  CRが調整されると（GitLabリソースのステータスが`Running`の場合）、`https://gitlab.example.com`でブラウザーでGitLabにアクセスできます。

ログインするには、デプロイの初期ルートパスワードを取得する必要があります。詳細な手順については、[Helmチャートドキュメント](https://docs.gitlab.com/charts/installation/deployment/#initial-login)を参照してください。

## 推奨される次のステップ {#recommended-next-steps}

インストールが完了したら、[推奨される次のステップ](https://docs.gitlab.com/install/next_steps/)（認証オプションやサインアップ制限など）を実行することを検討してください。

### OpenShift {#openshift}

OpenShiftを実行する場合は、GitLabオペレーターの承認戦略を自動（デフォルト）から手動に変更します。これにより、[承認が得られる](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators)まで、OpenShiftが新しいオペレーターバージョンをインストールするのを防ぐことができます。

カスタム[`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster)を設定して、オペレーターのバージョンを固定したり、最新以外のバージョンにアップグレードしたりすることもできます。

- 承認戦略は、[OpenShift Webコンソール](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf)から変更するか、[サブスクリプションを編集](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm)して変更できます。
- 手動アップグレードを承認するには、`InstallPlan`の`.spec.approved`を`true`に設定します。
- 各GitLabオペレーターは、定義されたGitLabチャートバージョンのサブセットをサポートしています。オペレーターへのアップグレードでは、GitLabカスタムリソースのチャートバージョンも更新する必要があります。
- GitLabオペレーターと指定されたGitLab Helmチャートバージョンに互換性がない場合、チャートへの設定変更が[GitLab Helmチャートバージョンに関するエラー](operator_upgrades.md)で失敗する可能性があります。

{{< alert type="note" >}}

[OLMは、オペレーターのダウングレードをサポートしていません](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177)。

{{< /alert >}}

## GitLabオペレーターをアンインストールする {#uninstall-the-gitlab-operator}

GitLabオペレーターとその関連リソースを削除するには、次の手順に従います。

オペレーターをアンインストールする前に注意すべき点:

- オペレーターは、GitLabインスタンスが削除されても、永続ボリュームクレームまたはシークレットを削除しません。
- オペレーターを削除すると、インストールされているネームスペース（デフォルトでは`gitlab-system`）は自動的に削除されません。これにより、永続ボリュームが誤って失われることがなくなります。

### GitLabインスタンスをアンインストールする {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

これにより、GitLabインスタンスと、上記のように永続ボリュームクレームを除くすべての関連オブジェクトが削除されます）。

### GitLabオペレーターをアンインストールする {#uninstall-the-gitlab-operator-1}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

これにより、オペレーターの実行中のデプロイを含む、オペレーターのリソースが削除されます。これは、GitLabインスタンスに関連付けられたオブジェクトを削除**しません**。

## GitLabオペレーターのトラブルシューティング {#troubleshoot-the-gitlab-operator}

オペレーターのトラブルシューティングは、[トラブルシューティング.md](troubleshooting.md)にあります。
