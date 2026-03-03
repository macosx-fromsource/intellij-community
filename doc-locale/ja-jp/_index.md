---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Operator
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

[GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator)は、[Kubernetes Operatorパターン](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)に従うインストールおよび管理方法です。

GitLab Operatorを使用して、[OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/)または別のKubernetes互換プラットフォームでGitLabを実行します。

{{< alert type="note" >}}

GitLab Operatorには[既知の制限事項](#known-issues)があり、本番環境での使用における特定のシナリオにのみ適しています。

{{< /alert >}}

<!-- This warning block is duplicated in doc/installation.md. Changes should be reflected in both locations. -->

{{< alert type="warning" >}}

GitLabカスタムリソースのデフォルト値は、**not intended for production use**。これらの値を使用すると、GitLab Operatorは、永続データを含むすべてのサービスがKubernetesクラスターにデプロイされるGitLabインスタンスを作成します。これは、**not suitable for production workloads**。本番環境へのデプロイでは、[クラウドネイティブハイブリッドリファレンスアーキテクチャ](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)に従う**必要があります**。GitLabは、Kubernetesクラスター内にデプロイされたPostgreSQL、Redis、Gitaly、Praefect、またはMinIOに関連する問題はサポートしていません。

{{< /alert >}}

## 既知の問題 {#known-issues}

GitLab Operatorは、以下をサポートしていません:

- GitLab Operatorを使用した、既存のHelmチャートベースのインスタンスの管理。改善のサポートは、[GitLab Operatorイシュー1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567)で提案されています。
- [OpenShiftルート](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html)を使用したSSH経由のGit。詳細については、[OpenShiftルートに関するGitLab Operatorドキュメント](openshift_ingress.md#openshift-routes)を参照してください。
- 他のクラウドAPI（オブジェクトストレージなど）へのワークロードを認証するための[GKEワークロードID](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity)および[IAMサービスアカウント](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html)。詳細については、[GitLab Operatorイシュー1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737)を参照してください。
- GitLab Operatorは、ゼロダウンタイム方式を使用してGitLabをアップグレードします。その結果、GitLabとGitLabチャートのバージョンは、一度に1つのマイナーリリースずつ更新する必要があります。一度に複数のバージョンをアップグレードするためのサポートは、[GitLab Operatorイシュー1952](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1952)で追跡されています。

GitLab Operatorには、GitLabチャートの他の制限事項があります。GitLab Operatorは、KubernetesリソースをプロビジョニングするためにGitLabチャートに依存しています。したがって、GitLabチャートの制限は、GitLab Operatorに影響を与えます。GitLab OperatorからのGitLabチャートの依存関係を削除することは、[Cloud Nativeエピック64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64)で提案されています。

## インストール {#installation}

GitLab Operatorをインストールする方法については、[インストールドキュメント](installation.md)を参照してください。

[セキュリティコンテキスト制約](security_context_constraints.md)の使用方法の詳細は、それぞれのドキュメントに記載されています。

特にOpenShiftを使用する場合は、[SSHアクセスからGitへの考慮事項](git_over_ssh.md)も認識しておく必要があります。

## アップグレード {#upgrading}

GitLab OperatorまたはGitLab Operatorによって管理されるGitLabインスタンスをアップグレードする方法については、[GitLab Operatorを使用したGitLabインスタンスのアップグレード](gitlab_upgrades.md)を参照してください。

## バックアップと復元 {#backup-and-restore}

[バックアップと復元する](backup_and_restore.md)のドキュメントでは、Operatorによって管理されるGitLabインスタンスをバックアップおよび復元する方法について説明します。

## RedHat認定イメージの使用 {#using-redhat-certified-images}

[RedHat認定イメージ](certified_images.md)のドキュメントでは、RedHatによって認定されたイメージをデプロイするようにGitLab Operatorに指示する方法について説明します。

## デベロッパーツール {#developer-tooling}

- [デベロッパーガイド](developer/guide.md): プロジェクトの構造とコントリビュートする方法の概要。
- [バージョニングとリリースの情報](developer/releases.md): Operatorのバージョニングとリリースに関する注意事項を記録します。
- [設計上の決定](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr): このプロジェクトはアーキテクチャ上の決定記録を利用しており、このOperatorの構造、機能、および機能の実装について詳しく説明しています。

## マージリクエストのレビュー {#merge-request-reviews}

マージリクエスト（MR）は通常、2人のレビュアーを必要とする標準的な方法に従います。まず、メンテナー以外の人がMRをレビューし、提案されている変更を改善/修正するために、作成者にコメントを提供します。作成者が必要な更新を行い、レビュアーがMRを承認した後、メンテナーの1人にレビューをリクエストします。

このアプローチは、経験の浅いレビュアーに学習の機会を提供するだけでなく、経験の浅いレビュアーに学習の機会を提供します。最初のレビューでは、最終的なレビューの前に、MRのほとんどの問題に対処します。大量のプロジェクトでは、メンテナーの負荷によりボトルネックが発生することがよくありますが、この最初のパスはそれらの負荷を軽減するのに役立ちます。

### 1つの承認のみの例外 {#one-approval-only-exceptions}

特定の場合には、1つの承認のみでMRをマージすることを許可しています。

#### Goモジュールの更新 {#go-modules-updates}

**注:** これは、このプロジェクトを所有するグループのGitLabチームメンバーのみに関連します。

このプロジェクトを所有するチームのチームメンバーである場合は、`go.mod`ファイルと`go.sum`ファイルに対するCODEOWNERSの承認権限が付与されています。MRがこれらのファイルのみを変更している場合、メンテナーでなくても、MRを承認してマージできるはずです。これは、Goモジュールの更新のリスクが非常に低いとチームが評価したため、メンテナーからのレビューの負荷を軽減し、依存関係の更新効率性を向上させるために実装されました。そのため、Goに慣れていて、変更が適切に見え、MRで完全にグリーンパイプラインが表示されている場合は、すぐに承認してマージしてください。

それでも、Goコードに慣れていない場合、またはその他の理由で別の意見が必要な場合は、2回目のレビューを実行するために、MRをメンテナーに渡すことをお勧めします。
