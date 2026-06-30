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

[OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/)または別のKubernetes互換プラットフォームでGitLabを実行するには、GitLab Operatorを使用してください。

> [!note]
> GitLab Operatorには[既知の制限事項](#known-issues)があり、本番環境での特定のシナリオにのみ適しています。

GitLab Operatorには、外部の[PostgreSQL](https://docs.gitlab.com/charts/advanced/external-db/)、[Redis](https://docs.gitlab.com/charts/advanced/external-redis/)、および[オブジェクトストレージ](https://docs.gitlab.com/charts/advanced/external-object-storage/)が必要です。

本番環境へのデプロイでは、[クラウドネイティブリファレンスアーキテクチャ](https://docs.gitlab.com/administration/reference_architectures)に従ってください。

## 既知の問題 {#known-issues}

GitLab Operatorは、以下をサポートしていません:

- GitLab Operatorで既存のHelmチャートベースのインスタンスを管理すること。この改善に関するサポートは、[GitLab Operatorのイシュー1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567)で提案されています。
- [OpenShift Routes](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html)を使用したSSH経由のGit。詳細については、[OpenShift Routesに関するGitLab Operatorドキュメント](openshift_ingress.md#openshift-routes)を参照してください。
- ワークロードを他のクラウドAPI（オブジェクトストレージなど）に対して認証するための[GKEワークロードID](https://docs.cloud.google.com/kubernetes-engine/docs/concepts/workload-identity)および[IAMサービスアカウント](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html)。詳細については、[GitLab Operatorのイシュー1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737)を参照してください。
- デフォルトでは、OperatorはゼロダウンタイムでGitLabをアップグレードします。そのため、GitLabとGitLabチャートのバージョンは、マイナーリリースを1つずつ順に更新する必要があります。マイナーバージョンをスキップするには、[ゼロダウンタイムアップグレードを無効化](gitlab_upgrades.md#upgrade-with-downtime)できますが、アップグレード中にダウンタイムが発生します。

GitLab Operatorには、GitLabチャートのその他の制限事項が適用されます。GitLab Operatorは、KubernetesリソースをプロビジョニングするためにGitLabチャートに依存しています。したがって、GitLabチャートにおける制限はGitLab Operatorに影響を与えます。GitLab OperatorからGitLabチャートへの依存を解消することは、[クラウドネイティブのエピック64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64)で提案されています。

## インストール {#installation}

GitLab Operatorのインストール方法については、[インストールドキュメント](installation.md)を参照してください。

[セキュリティコンテキストの制約](security_context_constraints.md)をどのように使用しているかの詳細については、それぞれのドキュメントに記載しています。

特にOpenShiftを使用する場合は、[GitへのSSHアクセスに関する考慮事項](git_over_ssh.md)にも注意してください。

## アップグレード {#upgrading}

GitLab Operator、またはGitLab Operatorによって管理されるGitLabインスタンスをアップグレードする方法については、[GitLab OperatorでGitLabインスタンスをアップグレードする](gitlab_upgrades.md)を参照してください。

## バックアップと復元 {#backup-and-restore}

[バックアップと復元](backup_and_restore.md)のドキュメントでは、Operatorで管理されるGitLabインスタンスをバックアップおよび復元する方法について説明しています。

## RedHat認定イメージを使用する {#using-redhat-certified-images}

[RedHat認定イメージ](certified_images.md)のドキュメントでは、RedHatによって認定されたイメージをGitLab Operatorにデプロイさせる方法を説明しています。

## デベロッパーツール {#developer-tooling}

- [デベロッパーガイド](developer/guide.md): プロジェクトの構造とコントリビュートの方法の概要を説明しています。
- [バージョニングとリリースの情報](developer/releases.md): Operatorのバージョニングとリリースに関する注意事項を記録しています。
- [設計上の判断](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/tree/master/doc/developer/adr): このプロジェクトではアーキテクチャ決定レコードを使用しており、このOperatorの構造、機能、および機能の実装の詳細を記述しています。

## マージリクエストのレビュー {#merge-request-reviews}

マージリクエスト（MR）は通常、2人のレビュアーを必要とする標準的な運用に従います。まずメンテナー以外のメンバーがMRをレビューし、提案されている変更の改善/修正を支援するために作成者にコメントを提供します。作成者が必要な更新を行い、レビュアーがMRを承認した後、メンテナーの1人にレビューをリクエストします。

このアプローチは、経験の浅いレビュアーに学習の機会を提供します。最初のレビューでは、最終レビューの前にMRに関するほとんどの問題を解決します。変更の多いプロジェクトでは、メンテナーの負荷によりボトルネックが発生しがちですが、この最初のパスはそれらの負荷を軽減するのに役立ちます。

### 1回の承認のみの例外 {#one-approval-only-exceptions}

特定のケースでは、1回の承認のみでMRをマージすることを許可しています。

#### Goモジュールの更新 {#go-modules-updates}

> [!note]
> これは、このプロジェクトを所有するグループのGitLabチームメンバーにのみ関連します。

このプロジェクトを所有するチームのメンバーである場合、`go.mod`ファイルと`go.sum`ファイルに対するCODEOWNERS承認権限が付与されています。MRがこれらのファイルのみを変更している場合、メンテナーでなくてもMRを承認してマージできるはずです。これは、Goモジュールの更新は非常にリスクが低いとチームが評価したことを踏まえ、メンテナーによるレビューの負荷を軽減し、依存関係更新の効率性を向上させる目的で実装されました。そのため、Goに精通しており変更内容に問題がなく、MRのパイプラインがすべてグリーンであれば、すぐにそのまま承認してマージしてください。

ただし、Goコードにまだ精通していない、などの理由によりセカンドオピニオンを必要とする場合は、MRをメンテナーに回して2回目のレビューを実施することをおすすめします。
