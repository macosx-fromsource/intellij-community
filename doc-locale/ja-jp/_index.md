---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Operator
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

[GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator)は、[Kubernetes Operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)パターンに従うインストールおよび管理方法です。

[OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/)または他のKubernetes互換プラットフォームでGitLabを実行するには、GitLab Operatorを使用します。

{{< alert type="note" >}}

GitLabオペレーターには[既知の制限事項](#known-issues)があり、本番環境での使用における特定のシナリオにのみ適しています。

{{< /alert >}}

<!-- This warning block is duplicated in doc/installation.md. Changes should be reflected in both locations. -->

{{< alert type="warning" >}}

GitLabカスタムリソースのデフォルト値は、**本番環境での使用を意図していません**。これらの値を使用すると、GitLabオペレーターは、永続データを含むすべてのサービスがKubernetesクラスターにデプロイされるGitLabインスタンスを作成しますが、これは**本番環境のワークロードに適していません**。本番環境へのデプロイでは、[クラウドネイティブハイブリッドリファレンスアーキテクチャ](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)に従う**必要があります**。GitLabは、Kubernetesクラスター内にデプロイされたPostgreSQL、Redis、Gitaly、Praefect、またはMinIOに関連するイシューをサポートしていません。

{{< /alert >}}

## 既知の問題 {#known-issues}

GitLab Operatorは、以下をサポートしていません:

- GitLab Operatorを使用した、既存のHelmチャートベースのインスタンスの管理。改善のサポートは、[GitLab Operator issue 1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567)で提案されています。
- [OpenShift](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html)ルートを使用したSSH経由のGit。詳細については、[OpenShiftルート](openshift_ingress.md#openshift-routes)に関するGitLab Operatorのドキュメントを参照してください。
- 他のクラウドAPI（オブジェクトストレージなど）へのワークロードを認証するための[GKEワークロードID](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity)および[IAMサービスアカウント](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html)。詳細については、[GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737)のイシュー1089を参照してください。

GitLab Operatorには、GitLabチャートの他の制限があります。GitLab Operatorは、KubernetesリソースをプロビジョニングするためにGitLabチャートに依存しています。したがって、GitLabチャートの制限は、GitLab Operatorに影響を与えます。GitLab OperatorからのGitLabチャートの依存関係の削除は、[Cloud Nativeエピック64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64)で提案されています。

## インストール {#installation}

GitLab Operatorをインストールする方法については、[インストールドキュメント](installation.md)を参照してください。

[セキュリティコンテキスト制約](security_context_constraints.md)の使用方法の詳細は、それぞれのドキュメントに記載されています。

特にOpenShiftを使用する場合は、[SSHアクセスからGit](git_over_ssh.md)への考慮事項も把握しておく必要があります。

## アップグレード {#upgrading}

[Operatorのアップグレード](operator_upgrades.md)のドキュメントでは、GitLab Operatorをアップグレードする方法について説明しています。

[GitLabのアップグレード](gitlab_upgrades.md)のドキュメントでは、GitLabインスタンスをアップグレードする方法について説明しています（GitLab Operatorによって管理）。

## バックアップと復元 {#backup-and-restore}

[バックアップとリストア](backup_and_restore.md)のドキュメントでは、Operatorによって管理されているGitLabインスタンスをバックアップおよびリストアする方法について説明しています。

## RedHat認定イメージ {#using-redhat-certified-images}

[RedHat認定イメージ](certified_images.md)のドキュメントでは、RedHatによって認定されたイメージをデプロイするようにGitLab Operatorに指示する方法について説明します。

## デベロッパーツール {#developer-tooling}

- [開発者ガイド](developer/guide.md): プロジェクトの構成とコントリビュートする方法の概要。
- [バージョニング](developer/releases.md)とリリース情報: オペレーターのバージョニングとリリースに関する注意事項を記録します。
- [設計上の判断](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr): このプロジェクトでは、アーキテクチャの決定レコードを利用して、このオペレーターの構造、機能、および機能の実装について詳しく説明します。

## マージリクエストのレビュー {#merge-request-reviews}

マージリクエスト（MR）は通常、2人のレビュアーを必要とする標準的な方法に従います。まず、メンテナー以外の担当者がMRをレビューし、作成者にコメントを提供して、提案されている変更の改善/修正を支援します。作成者が必要な更新を行い、レビュアーがMRを承認すると、メンテナーの1人からのレビューをリクエストします。

このアプローチは、経験の浅いレビュアーに学習機会を提供するだけでなく、このアプローチは、経験の浅いレビュアーに学習機会を提供します。最初のレビューでは、最終的なレビューの前に、MRのほとんどの問題に対処します。大量のプロジェクトでは、メンテナーの負荷が原因でボトルネックが発生することがよくありますが、この最初のパスは負荷を軽減するのに役立ちます。

### 1つの承認のみの例外 {#one-approval-only-exceptions}

特定の場合には、1つの承認のみでマージ（MR）をマージできるようにします。

#### Goモジュールのアップデート {#go-modules-updates}

**ノート:** これは、このプロジェクトを所有するグループのGitLabチームメンバーにのみ関連します。

あなたがこのプロジェクトを所有するチームのチームメンバーである場合、`go.mod`ファイルと`go.sum`ファイルのCODEOWNERS承認権限が付与されました。MRがこれらのファイルのみを変更する場合、メンテナーでなくても、MRを承認してマージできるはずです。これは、Goモジュールのアップデートのリスクが非常に低いとチームが評価したことを考えると、メンテナーからのレビュー負荷を軽減し、依存関係のアップデートの効率性を向上させるために実装されました。そのため、Goに慣れていて、変更が適切に見え、MRに完全にグリーンパイプラインがある場合は、すぐに承認してマージしてください。

それでも、Goコードに慣れていない場合、またはその他の理由で2回目の意見が必要な場合は、2回目のレビューを実行するために、MRをメンテナーに渡すことをお勧めします。
