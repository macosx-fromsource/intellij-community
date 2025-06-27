---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Operator
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 提供:GitLab Self-Managed

{{< /details >}}

[GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator)は、[Kubernetes Operatorパターン](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)に従うインストールおよび管理方法です。

GitLab Operatorを使用して、[OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/)または別のKubernetes互換プラットフォームでGitLabを実行します。

{{< alert type="note" >}}

GitLab Operatorには[既知の制限事項](#known-issues)があり、本番環境での特定シナリオにのみ適しています。

{{< /alert >}}

<!-- This warning block is duplicated in doc/installation.md. Changes should be reflected in both locations. -->

{{< alert type="warning" >}}

_GitLabカスタムリソース_のデフォルト値は、**本番環境での使用を想定していません**。これらの値を使用すると、GitLab Operatorは、永続データを含む_すべて_のサービスがKubernetesクラスターにデプロイされるGitLabインスタンスを作成します。これは**本番環境ワークロードには適していません**。本番環境デプロイメントの場合、**必ず**[クラウドネイティブハイブリッド参照アーキテクチャ](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)に従ってください。GitLabは、Kubernetesクラスター内にデプロイされたPostgreSQL、Redis、Gitaly、Praefect、またはMinIOに関連する問題はサポートしていません。

{{< /alert >}}

## 既知の問題 {#known-issues}

GitLab Operatorは以下をサポートしていません。

- GitLab ChartまたはLinuxパッケージからGitLab Operatorへの移行。インストール方法の移行については、[手動移行手順](https://docs.gitlab.com/charts/installation/migration/package_to_helm/)と同様の手順に従う必要があります。自動移行のサポートは、[GitLab Operatorイシュー1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567)で提案されています。
- [OpenShiftルート](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html)でのSSH経由のGit。詳細については、[OpenShiftルートに関するGitLab Operatorドキュメント](openshift_ingress.md#openshift-routes)を参照してください。
- [GKEワークロードアイデンティティ](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity)および[IAMサービスアカウント](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html)を使用して、ワークロードを他のクラウドAPI（オブジェクトストレージなど）に認証します。詳細については、[GitLab Operatorイシュー1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737)を参照してください。

GitLab Operatorには、GitLabチャートの他の制限事項があります。GitLab Operatorは、KubernetesリソースをプロビジョニングするためにGitLabチャートに依存しています。したがって、GitLabチャートの制限事項はGitLab Operatorに影響します。GitLab OperatorからのGitLabチャートの依存関係の削除は、[Cloud Nativeエピック64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64)で提案されています。

## インストール {#installation}

GitLab Operatorのインストール方法については、[インストールに関するドキュメント](installation.md)を参照してください。

[セキュリティコンテキスト制約](security_context_constraints.md)の使用方法の詳細は、それぞれのドキュメントに記載されています。

特にOpenShiftを使用する場合は、[GitへのSSHアクセスに関する考慮事項](git_over_ssh.md)も認識しておく必要があります。

## アップグレード {#upgrading}

[Operatorのアップグレード](operator_upgrades.md)に関するドキュメントでは、GitLab Operatorをアップグレードする方法について説明します。

[GitLabのアップグレード](gitlab_upgrades.md)に関するドキュメントでは、GitLab Operatorによって管理されるGitLabインスタンスをアップグレードする方法について説明します。

## バックアップと復元 {#backup-and-restore}

[バックアップと復元](backup_and_restore.md)に関するドキュメントでは、Operatorによって管理されるGitLabインスタンスをバックアップおよび復元する方法について説明します。

## RedHat認定イメージの使用 {#using-redhat-certified-images}

[RedHat認定イメージ](certified_images.md)に関するドキュメントでは、GitLab OperatorにRedHatによって認定されたイメージをデプロイするように指示する方法について説明します。

## デベロッパーツール {#developer-tooling}

- [デベロッパーガイド](developer/guide.md):プロジェクトの構造とコントリビュートする方法について概説します。
- [バージョニングとリリース情報](developer/releases.md):Operatorのバージョニングとリリースに関するノートを記録します。
- [設計に関する決定](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr):このプロジェクトでは、アーキテクチャに関する意思決定記録を利用して、このOperatorの構造、機能、および機能の実装について詳しく説明します。
- [OpenShiftクラスターのセットアップ](developer/openshift_cluster_setup.md):*開発*目的でOpenShiftクラスターを作成/構成する手順。
