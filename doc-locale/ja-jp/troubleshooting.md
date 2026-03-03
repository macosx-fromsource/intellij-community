---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Operatorのトラブルシューティング
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

このドキュメントは、GitLab Operatorのインストール、およびGitLabカスタムリソースからのGitLabインスタンスのデプロイのトラブルシューティングを支援するためのメモとヒントを集めたものです。

## インストールに関する問題 {#installation-problems}

Kubernetes環境でのOperatorのインストールのトラブルシューティングは、他のKubernetesワークロードのトラブルシューティングとよく似ています。Operatorマニフェストをデプロイした後、Operatorポッドの`kubectl describe`または`kubectl get events -n <namespace>`の出力を監視します。これにより、Operatorイメージの取得に関する問題や、Operatorの起動に関するその他の前提条件が示されます。

Operatorが起動しているにもかかわらず、途中で終了する場合は、Operatorログを調べると、ポッドの終了原因を特定するための情報が得られます。これは、次のコマンドで実行できます:

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

さらに、Operatorは、適切な動作のためにTLS証明書を作成するために、Cert Managerに依存しています。TLS証明書は、シークレットとして作成され、Operatorポッドのボリュームとしてマウントされます。TLS証明書の取得に関する問題は、ネームスペースのイベントログにあります。

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

次のステップでは、TLS証明書の作成の失敗を示す問題をCert Managerログで調べます。

### OpenShift固有の問題 {#openshift-specific-problems}

OpenShiftは、より制限の厳しいセキュリティモデルを採用しているため、GitLab Operatorはクラスター管理者アカウントでインストールする必要があります。開発者アカウントには、Operatorが適切に機能するために必要な権限がありません。

[Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx)ポッドが[このイシュー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762)で説明されているように無効なSCCパラメータによりプロビジョニングできない場合、適切な回避策は、Ingress NGINX ControllerがOpenShiftクラスターで起動できるように、リポジトリからSCCを更新することです:

1. GitLab Operator用の[最新のOpenShiftマニフェスト](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)をフェッチします。`gitlab-operator-openshift-VERSION.yaml`が必要です
1. SCCを抽出

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. `scc.yaml`をクラスターに適用します:

   ```shell
   kubectl apply -f scc.yaml
   ```

GitLab Operatorリポジトリのリリースページからリリースされたマニフェストからインストールすると、SCCが含まれているため、この問題は発生しません。OperatorHubでサポートされていないオブジェクトに関する関連イシュー:

- [IngressClass CRのサポートを追加 · イシュー5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [OpenShiftのSCCのサポートを追加 · イシュー2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## GitLabインスタンスのデプロイに関する問題 {#problems-with-deployment-of-gitlab-instance}

ここに記載されている情報に加えて、GitLab Helmチャートの[トラブルシューティングドキュメント](https://docs.gitlab.com/charts/troubleshooting/)を参照してください。

### コアサービスが準備できていません {#core-services-not-ready}

GitLab Operatorは、Redis、PostgreSQL、Gitalyのインスタンスのインストールに依存しており、これらはコアサービスとして知られています。GitLabカスタムリソースをデプロイした後、コアサービスが準備できていないことを示すOperatorログメッセージが過剰に表示される場合は、これらのサービスのいずれかで動作に問題が発生しています。

これらの各サービスのエンドポイントを具体的にチェックして、サービスポッドに接続されていることを確認します。これは、クラスターにGitLabインスタンスをサポートするのに十分なリソースがない可能性も示しており、クラスターにノードを追加する必要があります。

イシュー[\#305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305)が作成され、どのコアサービスがGitLabインスタンスのデプロイを停止させているかのレポートを追跡しています。

### GitLab UIにアクセスできない（Ingressにアドレスがない、またはCertManager Challengeが失敗する） {#gitlab-ui-unreachable-ingresses-have-no-address-andor-certmanager-challenges-failing}

GitLab OperatorのインストールマニフェストとHelm Chartは、`nameOverride`がHelmの値で指定されていない限り、デフォルトで、すべてのリソース名のプレフィックスとして`gitlab`を使用します。

その結果、NGINX IngressClassの名前は`gitlab-nginx`になります。`metadata.name`でGitLabカスタムリソースに`gitlab`以外のリリース名が指定されている場合は、`global.ingress.class`でデフォルトのIngressClass名を明示的に設定する必要があります:

例: `metadata.name`が`demo`に設定されている場合は、`global.ingress.class=gitlab-nginx`を設定します:

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

この明示的な設定がないと、Ingressは`demo-nginx`という名前のIngressを検索しようとしますが、これは存在しません。

### NGINX Ingress Controllerポッドが見つからない {#nginx-ingress-controller-pods-missing}

OpenShift環境では、[NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)が、GitLabインスタンス（HTTPSとSSHの両方）へのトラフィックを誘導するために、OpenShiftルートの代わりに使用されます。GitLabインスタンスへの接続で問題が発生している場合は、まず、NGINX Ingress Controllerのデプロイがあることを確認してください。

デプロイが存在する場合は、`kubectl get deploy`出力の`READY`列を確認します。`READY`ステータスが`0/0`としてレポートされている場合は、`kubectl get events -n <namespace> | grep -i nginx`の出力を調べて、Security Context Constraint（SCC）が侵害されたことを示すメッセージを探します。

これは、OpenShift用のNGINX RBACリソースがデプロイされていないことを示しています。OpenShift用のOperatorマニフェストは、次のコマンドで再適用する必要があります:

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

マニフェストが適用されたら、SCCを適切に取得し、Ingressコントローラーがポッドを正しく作成できるようにするために、Ingressコントローラーデプロイを削除する必要がある場合があります。

### 水平ポッドオートスケーラーがスケールしていません {#horizontal-pod-autoscalers-are-not-scaling}

水平ポッドオートスケーラー（HPA）がトラフィック負荷に応じてポッドの数をスケールしないことが判明した場合は、Metrics Serverのインストールを確認してください。Kubernetesクラスターでは、Metrics Serverは、インストールする必要がある追加のコンポーネントです。インストールプロセスは、[インストールドキュメント](installation.md#metrics)に記載されています。

OpenShiftクラスターにはMetrics Serverが組み込まれているため、HPAは正しく動作するはずです。

### PersistentVolumeClaimの設定が変更された場合のデータの復元 {#restoring-data-when-persistentvolumeclaim-configuration-changes}

データ永続化のためにMinIOなどのコンポーネントを使用する場合、以前のPersistentVolumeに再接続する必要がある場合があります。

たとえば、[!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419)は、Operator定義のMinIOコンポーネントをGitLab Helm ChartsのMinIOコンポーネントに置き換えました。この変更の一環として、PersistentVolumeClaimを含め、オブジェクト名が変更されました。その結果、OperatorにバンドルされているMinIOインスタンスを使用している人は誰でも、永続化されたデータを含む以前のPersistentVolumeに再接続するために追加の手順を実行する必要がありました。

GitLab Operator `0.6.4`にアップグレードした後、次の手順を完了して、新しいPersistentVolumeClaimを以前のPersistentVolumeに接続します:

1. `$RELEASE_NAME-minio-secret`シークレットを削除します。シークレットの内容は`0.6.4`のアップグレードで変更されますが、シークレット名は変更されません。
1. 以前のMinIO PersistentVolumeを編集し、`.spec.persistentVolumeReclaimPolicy`を`Delete`から`Retain`に変更します。
1. 以前のMinIO StatefulSet `$RELEASE_NAME-minio`を削除します。
1. 以前のMinIO PersistentVolumeClaimから切り離すために、以前のMinIO PersistentVolumeから`.spec.ClaimRef`を削除します。
1. 以前のMinIO PersistentVolumeClaim `export-gitlab-minio-0`を削除します。
1. 以前のPersistentVolumeのステータスが`Available`になったことを確認します。
1. GitLabカスタムリソースで次の値を設定します: `minio.persistence.volumeName=<previous PersistentVolume name>`。
1. GitLabカスタムリソースを適用します。
1. 新しいMinIO PersistentVolumeClaim（およびMinIOポッド）を削除して、PersistentVolumeClaimのバインドを解除して削除できるようにします。OperatorはPersistentVolumeClaimを再作成します。これは、`.spec`フィールドがイミュータブルであるために必要です。
1. 以前のMinIO PersistentVolumeが新しいMinIO PersistentVolumeClaimにバインドされていることを確認します。
1. GitLab UIでイシュー、アーティファクトなどに移動して、データが復元されたことを確認します。

以前のPersistentVolumeへの再接続の詳細については、[永続ボリュームに関するドキュメント](https://docs.gitlab.com/charts/advanced/persistent-volumes/)を参照してください。

念のため、バンドルされたMinIOインスタンスは[本番環境での使用は推奨されていません](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart)。

### 複数のデータベース接続を設定する {#configure-multiple-database-connections}

GitLab 16.0では、GitLabはデフォルトで、同じPostgreSQLデータベースを指す2つのデータベース接続を使用するようになっています。

単一のデータベース接続に切り替える場合は、[複数のデータベース接続の設定](https://docs.gitlab.com/charts/charts/globals/#configure-multiple-database-connections)を参照してください。

### コンポーネントの無効化または名前変更 {#disabling-or-renaming-components}

リソースの名前変更と無効化は`nameOverride`への変更とさまざまな`*.enable: false`値の組み合わせによって可能ですが、GitLab Operatorは不要になったKubernetesリソースを自動的に削除しません。その結果、上記の操作では、不要になったリソースを手動で管理する必要があります。

ただし、GitLabカスタムリソースのインスタンスを削除すると、そのインスタンスに関連付けられているすべてのリソースが予期どおりに削除されます。

イシュー[!889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889)が作成され、これの追跡を維持しています。
