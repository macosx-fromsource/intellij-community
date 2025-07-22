---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLabのバックアップと復元
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

このドキュメントでは、Toolboxを使用してGitLabインスタンスをバックアップおよび復元する方法について説明します。

## 一般的なバックアップと復元のガイダンス {#general-backup-and-restore-guidance}

Operatorは[Toolboxチャート](https://docs.gitlab.com/charts/charts/gitlab/toolbox/)をデプロイします。つまり、[GitLabインスタンスをバックアップおよび復元](https://docs.gitlab.com/charts/backup-restore/)する方法に関する既存のドキュメントは、Operatorにも適用できます。

## HelmベースのインストールとOperatorベースのインストール間の移行 {#migration-between-helm-based-and-operator-based-installations}

Helmベースのインストールで作成されたバックアップは通常、Operatorベースのインストールで復元でき、その逆も可能です。

これらのフローは、Operatorの開発全体を通して、より広範にテストされる予定です。詳細については、[イシュー#320](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/320)を参照してください。

PostgreSQLやGitalyなどのステートフルなコンポーネントに外部サービスを利用する環境は、一般的に移行が容易であることに注意してください。
