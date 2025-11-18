---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Operatorのトラブルシューティング
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

このドキュメントは、GitLab Operatorのインストール、およびGitLabカスタムリソースからのGitLabインスタンスのデプロイのトラブルシューティングを支援するためのメモとヒントを集めたものです。

## インストールの問題 {#installation-problems}

Kubernetes環境でのOperatorのインストールのトラブルシューティングは、他のKubernetesのワークロードのトラブルシューティングとよく似ています。Operatorのマニフェストをデプロイした後、Operatorポッドの`kubectl describe`または`kubectl get events -n <namespace>`の出力を監視します。これにより、Operatorイメージの取得に関する問題や、Operatorを起動するためのその他の前提条件が示されます。

Operatorが起動しているものの、途中で終了する場合は、Operatorログを調べることで、ポッドの終了原因を特定するための情報が得られます。これは、次のコマンドで実行できます:

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

さらに、Operatorは、適切な動作のためにTLS証明書を作成するためにCert Managerに依存しています。TLS証明書はシークレットとして作成され、Operatorポッド上のボリュームとしてマウントされます。TLS証明書の取得に関する問題は、ネームスペースのイベントログに記録されます。

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

次のステップでは、TLS証明書の作成失敗を示す問題がないか、Cert Managerのログを調べます。

### OpenShift固有の問題 {#openshift-specific-problems}

OpenShiftにはより制限の厳しいセキュリティモデルがあり、その結果、GitLab Operatorはクラスターの管理者アカウントでインストールする必要があります。デベロッパーアカウントには、Operatorが適切に機能するために必要な権限がありません。

[Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx)ポッドが[この問題](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762)で説明されているように無効なSCCパラメータのためにプロビジョニングできない場合、適切な回避策は、Ingress NGINXがOpenShiftクラスターで起動できるように、リポジトリからSCCを更新することです:

1. GitLab Operatorの[最新のOpenShiftマニフェスト](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)をフェッチします。`gitlab-operator-openshift-VERSION.yaml`が必要です
1. SCCを抽出

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. `scc.yaml`をクラスターに適用します:

   ```shell
   kubectl apply -f scc.yaml
   ```

GitLab Operatorリポジトリのリリースページからリリースされたマニフェストからインストールすると、SCCが含まれているため、この問題は発生しません。OperatorHubでサポートされていないオブジェクトに関する関連問題:

- [IngressClass CRのサポートを追加 · Issue #5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [OpenShiftのSCCのサポートを追加 · Issue #2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## GitLabインスタンスのデプロイに関する問題 {#problems-with-deployment-of-gitlab-instance}

ここに記載されている情報に加えて、GitLab Helmチャートの[トラブルシューティングドキュメント](https://docs.gitlab.com/charts/troubleshooting/)を参照してください。

### コアサービスが準備できていません {#core-services-not-ready}

GitLab Operatorは、Redis、PostgreSQL、Gitalyのインスタンスのインストールに依存しており、これらはコアサービスとして知られています。GitLabカスタマーリソースをデプロイした後、コアサービスが準備できていないことを示すOperatorログメッセージが過剰に表示される場合は、これらのサービスのいずれかが動作不能になっている問題が発生しています。

特に、これらの各サービスのエンドポイントをチェックして、サービスのポッドに接続されていることを確認してください。これは、クラスターにGitLabインスタンスをサポートするのに十分なリソースがなく、追加のノードをクラスターに追加する必要がある可能性も示しています。

イシュー[\#305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305)が作成され、どのコアサービスがGitLabインスタンスのデプロイを停止させているかのレポートを追跡しています。

### GitLab UIに到達できない（Ingressにアドレスがない、またはCertManager Challengesが失敗する） {#gitlab-ui-unreachable-ingresses-have-no-address-andor-certmanager-challenges-failing}

GitLab OperatorのインストールマニフェストとHelmチャートは、`nameOverride`がHelmの値で指定されていない限り、`gitlab`をすべてのリソース名のプレフィックスとしてデフォルトで使用します。

その結果、NGINX IngressClassの名前は`gitlab-nginx`になります。`gitlab`以外のリリース名が`metadata.name`のGitLabカスタムリソースで指定されている場合は、デフォルトのIngressClass名を`global.ingress.class`で明示的に設定する必要があります:

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

この明示的な設定がない場合、Ingressは`demo-nginx`という名前のIngressを検索しようとしますが、これは存在しません。

### NGINX Ingress Controllerポッドが見つからない {#nginx-ingress-controller-pods-missing}

OpenShift環境では、[NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)が、GitLabインスタンス（HTTPSとSSHの両方）へのトラフィックを転送するために、OpenShiftルートの代わりに使用されます。GitLabインスタンスへの接続で問題が発生している場合は、まず、NGINX Ingress Controllerのデプロイがあることを確認してください。

デプロイが存在する場合は、`kubectl get deploy`出力の`READY`列を確認してください。`READY`ステータスが`0/0`としてレポートされている場合は、セキュリティコンテキスト制約（SCC）が違反していることを示すメッセージがないか、`kubectl get events -n <namespace> | grep -i nginx`の出力を調べてください。

これは、OpenShiftのNGINX RBACリソースがデプロイされなかったことを示しています。OpenShift用のOperatorマニフェストは、次のコマンドで再適用する必要があります:

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

マニフェストが適用されたら、SCCを適切に取得し、Ingressコントローラーがポッドを正しく作成できるようにするために、Ingressコントローラーデプロイを削除する必要がある場合があります。

### 水平ポッドオートスケーラーがスケールしていません {#horizontal-pod-autoscalers-are-not-scaling}

水平ポッドオートスケーラー（HPA）が、トラフィック負荷に応じてポッド数をスケールしないことが判明した場合は、Metrics Serverのインストールを確認してください。Kubernetesクラスターでは、Metrics Serverはインストールする必要がある追加のコンポーネントです。インストールプロセスは、[インストールドキュメント](installation.md#metrics)に記載されています。

OpenShiftクラスターにはMetrics Serverが組み込まれているため、HPAは正しく動作するはずです。

### PersistentVolumeClaimの設定が変更された場合のデータの復元 {#restoring-data-when-persistentvolumeclaim-configuration-changes}

データの永続化のためにMinIOなどのコンポーネントを使用する場合は、以前のPersistentVolumeに再接続する必要がある場合があります。

たとえば、[!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419)は、Operatorで定義されたMinIOコンポーネントを、GitLab HelmチャートのMinIOコンポーネントに置き換えました。この変更の一環として、オブジェクト名が変更され、PersistentVolumeClaimも含まれます。その結果、OperatorにバンドルされたMinIOインスタンスを使用している人は、永続化されたデータを含む以前のPersistentVolumeに再接続するために、追加の手順を実行する必要がありました。

GitLab Operator `0.6.4`にアップグレードしたら、次の手順を完了して、新しいPersistentVolumeClaimを以前のPersistentVolumeに接続します:

1. `$RELEASE_NAME-minio-secret`シークレットを削除します。シークレットの内容は`0.6.4`のアップグレードで変更されますが、シークレット名は変更されません。
1. 以前のMinIO PersistentVolumeを編集し、`.spec.persistentVolumeReclaimPolicy`を`Delete`から`Retain`に変更します。
1. 以前のMinIO StatefulSet `$RELEASE_NAME-minio`を削除します。
1. 以前のMinIO PersistentVolumeから`.spec.ClaimRef`を削除して、以前のMinIO PersistentVolumeClaimから関連付けを解除します。
1. 以前のMinIO PersistentVolumeClaim `export-gitlab-minio-0`を削除します。
1. 以前のPersistentVolumeステータスが`Available`になったことを確認します。
1. GitLabカスタムリソースに次の値を設定します: `minio.persistence.volumeName=<previous PersistentVolume name>`。
1. GitLabカスタムリソースを適用します。
1. 新しいMinIO PersistentVolumeClaim（およびMinIOポッド）を削除して、PersistentVolumeClaimがアンバインドされ、削除できるようにします。OperatorはPersistentVolumeClaimを再作成します。これは、`.spec`フィールドがイミュータブルであるために必要です。
1. 以前のMinIO PersistentVolumeが新しいMinIO PersistentVolumeClaimにバインドされていることを確認します。
1. データが復元されたことを確認するには、GitLab UIでイシュー、アーティファクトなどを参照します。

以前のPersistentVolumeへの再接続の詳細については、[永続ボリュームドキュメント](https://docs.gitlab.com/charts/advanced/persistent-volumes/)を参照してください。

念のため、バンドルされたMinIOインスタンスは[本番環境での使用は推奨されていません](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart)。

### 複数のデータベース接続を設定する {#configure-multiple-database-connections}

GitLab 16.0では、GitLabはデフォルトで、同じPostgreSQLデータベースを指す2つのデータベース接続を使用するようになっています。

単一データベース接続に戻す場合は、[複数データベース接続の設定](https://docs.gitlab.com/charts/charts/globals/#configure-multiple-database-connections)を参照してください。

### コンポーネントの無効化または名前変更 {#disabling-or-renaming-components}

リソースの名前変更と無効化は、`nameOverride`への変更とさまざまな`*.enable: false`値の組み合わせによって可能ですが、GitLab Operatorは不要になったKubernetesリソースを自動的に削除しません。その結果、上記の操作では、不要になったリソースの手動管理が必要になります。

ただし、GitLabカスタムリソースのインスタンスを削除すると、予想どおり、そのインスタンスに関連付けられているすべてのリソースが削除されます。

イシュー[!889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889)が作成され、これを追跡しています。
