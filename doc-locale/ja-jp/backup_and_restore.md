---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLabのバックアップと復元
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

GitLab Operatorは、[Toolboxチャート](https://docs.gitlab.com/charts/charts/gitlab/toolbox/)をデプロイします。Toolboxを使用してGitLabインスタンスをバックアップおよび復元するには、[バックアップとGitLabの復元](https://docs.gitlab.com/charts/backup-restore/)を参照してください。

## HelmベースのインストールとOperatorベースのインストール間で移行する {#migrate-between-helm-based-and-operator-based-installations}

Operatorベースのインスタンスのバックアップから新しいHelmチャートベースのインスタンスを作成したり、Helmチャートベースのインスタンスのバックアップから新しいOperatorベースのインスタンスを作成したりできます。

PostgreSQLやGitalyなどのステートフルコンポーネントに外部サービスを使用する環境は、通常、移行が容易です。
