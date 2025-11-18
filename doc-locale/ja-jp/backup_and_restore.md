---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLabのバックアップと復元
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

GitLab Operatorは、[Toolboxチャート](https://docs.gitlab.com/charts/charts/gitlab/toolbox/)をデプロイします。Toolboxを使用してインスタンスをバックアップおよび復元するするには、[バックアップおよび復元するGitLab](https://docs.gitlab.com/charts/backup-restore/)を参照してください。

## HelmベースのインストールとOperatorベースのインストール間の移行 {#migrate-between-helm-based-and-operator-based-installations}

Operatorベースのインスタンスのバックアップから新しいHelmチャートベースのインスタンスを作成したり、Helmチャートベースのインスタンスのバックアップから新しいOperatorベースのインスタンスを作成したりできます。

PostgreSQLやGitalyなどのステートフルなコンポーネントに外部サービスを利用する環境は、一般的に移行が容易です。
