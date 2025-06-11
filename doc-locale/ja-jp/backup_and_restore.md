---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLabのバックアップと復元
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 製品: GitLab Self-Managed

{{< /details >}}

このドキュメントでは、Toolboxを使用してGitLabインスタンスをバックアップしたり復元したりする方法について説明します。

## 一般的なバックアップと復元のガイダンス

Operatorは[Toolboxチャート](https://docs.gitlab.com/charts/charts/gitlab/toolbox/)をデプロイします。つまり、[GitLabインスタンスをバックアップしたり復元したり](https://docs.gitlab.com/charts/backup-restore/)する方法に関する既存のドキュメントは、Operatorにも当てはまります。

## HelmベースのインストールとOperatorベースのインストール間の移行

Helmベースのインストールで作成されたバックアップは、多くの場合、Operatorベースのインストールで復元することができます。その逆も可能です。

これらのフローは、Operatorの開発全体を通してより広範にテストされます。詳細については、[イシュー#320](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/320)を参照してください。

PostgreSQLやGitalyなどのステートフルコンポーネントのために外部サービスを活用する環境では、多くの場合、簡単に移行できることに注意してください。
