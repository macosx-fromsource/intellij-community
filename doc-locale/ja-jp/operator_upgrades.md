---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Operatorのアップグレード
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 製品:GitLab Self-Managed

{{< /details >}}

以下は、GitLab Operatorをアップグレードする手順です。

アップグレードする前に、[バックアップを実行](https://docs.gitlab.com/charts/backup-restore/)することを強くお勧めします。

## ステップ1:利用可能な最新のチャートバージョンにアップグレードする {#step-1-upgrade-to-the-latest-available-chart-version}

Operatorをアップグレードする前に、[GitLabアップグレードガイド](gitlab_upgrades.md)に従って、GitLabの現在のインスタンスが利用可能な最新のチャートバージョンにアップグレードされていることを確認してください。これらのバージョンは、`Version mapping`の見出しの下の[リリース](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)ページに概説されています。

たとえば、現在のOperatorバージョンが[リリース0.4.0](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases/0.4.0)の場合、利用可能なチャートのアップグレードを_順番に_実行します。`5.5.3` -> `5.6.3` -> `5.7.0`。

## ステップ2:目的のOperatorバージョンを特定する {#step-2-identify-the-desired-operator-version}

次にアップグレードするGitLabのバージョンを特定するには、[GitLabのアップグレードパス](https://docs.gitlab.com/update/#upgrade-paths)を参照し、目的のGitLabバージョンをサポートするGitLab Operatorバージョンを特定します。

GitLab Operatorの利用可能なバージョンの完全なリストについては、[リリース](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)ページを参照してください。

たとえば、現在のGitLab Operatorのバージョンが[`0.4.0`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases/0.4.0)で、アップグレード先のGitLabのバージョンが`14.7.1`(チャートバージョン`5.7.1`)の場合、[リリース0.4.1](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases/0.4.1)にアップグレードできます。

## ステップ3:目的のバージョンをインストールする {#step-3-install-the-desired-version}

次のステップでは、`kubectl`を使用して、目的のバージョンのOperatorのmanifestを適用します。

```shell
VERSION=X.Y.Z
kubectl apply -f \
  https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
```

このコマンドは、使用する新しいDeploymentイメージなど、関連するmanifestに対するすべての変更を適用します。

## ステップ4:新しいバージョンのOperatorがリーダーになることを確認する {#step-4-confirm-that-the-new-version-of-the-operator-becomes-the-leader}

Operatorのデプロイメントは、この変更により新しいReplicaSetを作成し、新しいOperatorを起動します。一方、以前のOperatorポッドはシャットダウンし、リーダーの状態を放棄します。この状態が発生すると、新しいOperatorポッドがリーダーになります。

## ステップ5:GitLabカスタムリソース(CR)のチャートバージョンを更新する {#step-5-update-the-chart-version-in-the-gitlab-custom-resource-cr}

ほとんどの場合、利用可能なチャートのバージョンは、Operatorのバージョン間で同一ではありません。新しいバージョンのOperatorが起動すると、既存のGitLabカスタムリソース(CR)を調整しようとします。次のようなエラーが発生する可能性があります:

```plaintext
Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
```

これに対処するには、そのリリースの利用可能なチャートバージョンから有効なバージョンを特定します。

例: Operator `0.4.0`から`0.4.1`にアップグレードする場合は、GitLab CRを利用可能なチャートバージョン`5.7.0`に最も近いバージョンに更新します。この場合は`5.7.1`です。

## ステップ6:OperatorがGitLabを期待どおりに調整することを確認する {#step-6-confirm-that-the-operator-reconciles-gitlab-as-expected}

新しいOperatorからのログを監視します。定義したチャートバージョンへのアップグレードが実行されていることを確認する必要があります。

アップグレードが成功したことを確認するには、GitLab CRの状態を取得します:

```plaintext
$ kubectl get gitlabs -n gitlab-system
NAME     STATUS    VERSION
gitlab   Running   5.7.1
```

状態`Running`は、Operatorがインスタンスへの変更を調整できたことを意味します。バージョンは、Operatorのアップグレード後に指定されたチャートバージョンと一致する必要があります。

エラーが発生した場合は、最初に[トラブルシューティング](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/troubleshooting.md)ドキュメントを参照してください。そこに答えがない場合は、既存の[イシュー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)を確認するか、[イシュートラッカー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)に新しい[イシュー](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)を登録してください。

## 関連資料 {#related-reading}

以下は、GitLabのアップグレードに関連するリソースです。

- [(Operator) GitLabのアップグレード](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/gitlab_upgrades.md)
- [(Charts) アップグレードガイド](https://docs.gitlab.com/charts/installation/upgrade/)
- [(Charts) バージョンのマッピング](https://docs.gitlab.com/charts/installation/version_mappings/)
- [GitLabのアップグレードパス](https://docs.gitlab.com/update/#upgrade-paths)
- [GitLabのバージョン固有の変更](https://docs.gitlab.com/update/package/#version-specific-changes)
- [GitLabのバージョン間の変更](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
