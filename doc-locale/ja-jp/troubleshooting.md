---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Operatorのトラブルシューティング
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 提供:GitLab Self-Managed

{{< /details >}}

このドキュメントは、GitLab Operatorのインストール、およびGitLabカスタムリソースからのGitLabインスタンスのデプロイのトラブルシューティングを支援するためのノートとヒントを集めたものです。

## インストールの問題 {#installation-problems}

Kubernetes環境へのOperatorのインストールに関するトラブルシューティングは、他のKubernetesワークロード のトラブルシューティングとよく似ています。Operatorのmanifestをデプロイした後、Operator Podの`kubectl describe`または`kubectl get events -n <namespace>`の出力を監視します。これにより、Operatorイメージの取得に関する問題や、Operatorの起動に関するその他の前提条件が示されます。

Operatorが起動しているにもかかわらず、途中で終了する場合は、Operatorのログを調べると、Podの終了原因を特定するための情報が得られます。これは、次のコマンドで実行できます。

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

また、Operatorは適切な動作のために、Cert Managerに依存してTLS証明書を作成します。TLS証明書はシークレットとして作成され、Operator Podのボリュームとしてマウントされます。TLS証明書の取得に関する問題は、ネームスペース のイベントログにあります。

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

次のステップでは、TLS証明書の作成失敗を示すイシュー を探して、Cert Managerのログを検査します。

### OpenShift固有の問題 {#openshift-specific-problems}

OpenShiftには、より制限の厳しいセキュリティモデルがあり、その結果、GitLab Operatorはクラスター 管理者アカウントでインストールする必要があります。デベロッパー アカウントには、Operatorが適切に機能するために必要な権限がありません。

[Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) Podが[このイシュー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762)で説明されているように無効なSCCパラメータによりプロビジョニング できない場合、適切な回避策は、リポジトリ からSCCを更新して、OpenShiftクラスター でIngress NGINXを起動できるようにすることです。

1. GitLab Operatorの[最新のOpenShift manifest](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)を取得します。`gitlab-operator-openshift-VERSION.yaml`が必要です
1. SCCを抽出します

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. `scc.yaml`をクラスター に適用します。

   ```shell
   kubectl apply -f scc.yaml
   ```

GitLab Operatorリポジトリ のReleasesページからリリースされたmanifestからインストールすると、SCCが含まれているため、この問題は発生しません。OperatorHubでサポートされていないオブジェクトに関する関連イシュー:

- [IngressClass CRのサポートを追加 · イシュー #5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [OpenShiftのSCCのサポートを追加 · イシュー #2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## GitLabインスタンスのデプロイに関する問題 {#problems-with-deployment-of-gitlab-instance}

ここに示されている情報に加えて、GitLab Helmチャート の[トラブルシューティング ドキュメント](https://docs.gitlab.com/charts/troubleshooting/)を参照する必要があります。

### コアサービスが準備できていません {#core-services-not-ready}

GitLab Operatorは、Redis、PostgreSQL、Gitalyのインスタンスのインストールに依存しており、これらはコアサービスとして知られています。GitLabカスタマー リソースをデプロイした後、コアサービスが準備できていないことを示すOperatorログメッセージが過剰にある場合、これらのサービスのいずれかで、運用可能になる上で問題が発生しています。

特に、これらの各サービスのエンドポイント をチェックして、サービスのポッド に接続されていることを確認してください。これは、クラスター にGitLabインスタンスをサポートするのに十分なリソースがなく、クラスター にノード を追加する必要がある可能性も示しています。

どのコアサービスがGitLabインスタンスのデプロイ を停止させているかのレポート を追跡するために、イシュー[\#305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305)が作成されました。

### GitLab UIに到達できません (Ingressにアドレスがない、またはCertManager Challengesが失敗する) {#gitlab-ui-unreachable-ingresses-have-no-address-andor-certmanager-challenges-failing}

GitLab OperatorのインストールmanifestとHelmチャート は、Helmの値で`nameOverride`が指定されていない限り、デフォルト で、すべてのリソース名のプレフィックス として`gitlab`を使用します。

その結果、NGINX IngressClassの名前は`gitlab-nginx`になります。`gitlab`以外のリリース名が`metadata.name`のGitLabカスタムリソースで指定されている場合、`global.ingress.class`でデフォルト のIngressClass名を明示的に設定する必要があります。

例: `metadata.name`が`demo`に設定されている場合は、`global.ingress.class=gitlab-nginx`を設定します。

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

### NGINX Ingress Controllerポッド が見つからない {#nginx-ingress-controller-pods-missing}

OpenShift環境では、[NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)は、GitLabインスタンスへのトラフィック (HTTPSとSSHの両方) を誘導するために、OpenShiftルート の代わりに使用されます。GitLabインスタンスへの接続で問題が発生している場合は、まず、NGINX Ingress Controllerのデプロイ が存在することを確認してください。

デプロイ が存在する場合は、`kubectl get deploy`出力の`READY`列を確認します。`READY`状態 が`0/0`として返された場合は、Security Context Constraint (SCC) が侵害されたことを示すメッセージを探して、`kubectl get events -n <namespace> | grep -i nginx`の出力を検査します。

これは、OpenShift用のNGINX RBACリソースがデプロイ されなかったことを示しています。OpenShift用のOperator manifestは、次のコマンドで再適用する必要があります。

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

manifestが適用されたら、SCCを適切に取得し、Ingressコントローラー がポッド を正しく作成できるようにするために、Ingressコントローラー デプロイメント を削除する必要がある場合があります。

### 水平ポッド オートスケーラー がスケーリング していません {#horizontal-pod-autoscalers-are-not-scaling}

水平ポッド オートスケーラー (HPA) がトラフィックの負荷に応じてポッド の数をスケーリング しないことが判明した場合は、Metrics Serverのインストールを確認してください。Kubernetesクラスター では、Metrics Serverはインストールする必要がある追加のコンポーネントです。インストール プロセスは、[インストール ドキュメント](installation.md#metrics)に記載されています。

OpenShiftクラスター にはMetrics Serverが組み込まれているため、HPAは正しく動作するはずです。

### PersistentVolumeClaim設定が変更された場合のデータの復元 {#restoring-data-when-persistentvolumeclaim-configuration-changes}

データの永続性のためにMinIOなどのコンポーネントを使用する場合、以前のPersistentVolumeに再接続する必要がある場合があります。

たとえば、[!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419)は、Operatorで定義されたMinIOコンポーネントを、GitLab Helmチャート のMinIOコンポーネントに置き換えました。この変更の一環として、PersistentVolumeClaimを含むオブジェクト 名が変更されました。その結果、Operatorバンドル されたMinIOインスタンスを使用しているすべてのユーザーは、永続化されたデータを含む以前のPersistentVolumeに再接続するために、追加の手順を実行する必要がありました。

GitLab Operator `0.6.4`にアップグレード した後、新しいPersistentVolumeClaimを以前のPersistentVolumeに接続するには、次の手順を実行します。

1. `$RELEASE_NAME-minio-secret`シークレット を削除します。シークレット の内容は`0.6.4`のアップグレード で変更されますが、シークレット 名は変更されません。
1. 以前のMinIO PersistentVolumeを編集し、`.spec.persistentVolumeReclaimPolicy`を`Delete`から`Retain`に変更します。
1. 以前のMinIO StatefulSet、`$RELEASE_NAME-minio`を削除します。
1. 以前のMinIO PersistentVolumeから`.spec.ClaimRef`を削除して、以前のMinIO PersistentVolumeClaimから切り離します。
1. 以前のMinIO PersistentVolumeClaim、`export-gitlab-minio-0`を削除します。
1. 以前のPersistentVolumeの状態 が`Available`になったことを確認します。
1. GitLabカスタム リソースで次の値を設定します: `minio.persistence.volumeName=<previous PersistentVolume name>`。
1. GitLabカスタム リソースを適用します。
1. 新しいMinIO PersistentVolumeClaim (およびMinIOポッド) を削除して、PersistentVolumeClaimがバインド解除され、削除できるようにします。OperatorはPersistentVolumeClaimを再作成します。これは、`.spec`フィールド がイミュータブルであるために必要です。
1. 以前のMinIO PersistentVolumeが新しいMinIO PersistentVolumeClaimにバインドされていることを確認します。
1. GitLab UIでイシュー、アーティファクト などに移動して、データが復元 されていることを確認します。

以前のPersistentVolumeへの再接続の詳細については、[永続ボリューム ドキュメント](https://docs.gitlab.com/charts/advanced/persistent-volumes/)を参照してください。

念のため、バンドル されたMinIOインスタンスは[本番環境 での使用は推奨されていません](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart)。

### 複数のデータベース接続を設定する {#configure-multiple-database-connections}

GitLab 16.0では、GitLabはデフォルト で、同じPostgreSQLデータベースを指す2つのデータベース接続を使用します。

単一のデータベース接続に切り替える場合は、[複数のデータベース接続の設定](https://docs.gitlab.com/charts/charts/globals/#configure-multiple-database-connections)を参照してください。

### コンポーネントの無効化または名前変更 {#disabling-or-renaming-components}

リソースの名前変更と無効化は、`nameOverride`への変更、およびさまざまな`*.enable: false`値の組み合わせによって可能ですが、GitLab Operatorは不要になったKubernetesリソースを自動的に削除しません。その結果、上記の操作を行うには、残りのリソースを手動で管理する必要があります。

ただし、GitLabカスタム リソースのインスタンスを削除すると、そのインスタンスに関連付けられているすべてのリソースが想定どおりに削除されます。

この追跡のために、イシュー[!889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889)が作成されました。
