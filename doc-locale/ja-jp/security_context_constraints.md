---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: セキュリティコンテキスト制約
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

## 概要 {#overview}

OpenShiftのポッドは、セキュリティコンテキスト制約に基づいて権限を付与されます。セキュリティコンテキスト制約（多くの場合、**SCC**と略されます）は、大規模なデプロイで使用できるように、ロールベースのアクセス制御の仕組みを簡素化します。[管理者は、アップストリームドキュメントを参照して、セキュリティコンテキスト制約の仕組みとOpenShiftにおける役割についてより深く理解することができます](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html)

管理者は、次のリソースも参照できます:

1. [OpenShiftでのセキュリティコンテキスト制約の管理](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [OpenShiftとUIDのガイド](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## GitLabデプロイ内のセキュリティコンテキスト制約 {#security-context-constraints-within-the-gitlab-deployment}

`gitlab-controller-manager`デプロイは、**Operator**プロセスを含むポッドを作成して管理します。このポッドと、同デプロイが作成して管理する他のポッドは、**restricted**セキュリティコンテキスト制約で実行されます。

**Operator**は、GitLabアプリケーションに必要なすべてのリソースを管理できる強力な権限を持つServiceAccountを使用します。

**Operator**は、クラウドネイティブGitLabを構成するコンポーネントサービスを管理します。**Operator**は、自身が指定したUIDに適合しないポッドを積極的に停止し、入れ替えます。この仕組みにより、最小権限の原則が強制されます。

### GitLabアプリケーションのカスタムリソース定義 {#gitlab-application-custom-resource-definitions}

GitLabカスタムリソースを満たすためにOperatorによってデプロイされるポッドは、**non-root-v2**セキュリティコンテキスト制約を使用します。サードパーティのOperatorおよびリソースのセキュリティコンテキスト制約については、[次のセクションで説明します](#third-party-resource-definitions)。

`gitlab-app-nonroot` ServiceAccountには付与された権限がなく、GitLabアプリケーションポッドに**nonroot-v2**セキュリティコンテキスト制約をバインドするためだけに存在します。

OpenShiftのセキュリティモデルにおいてGitLabアプリケーションの完全な読み取り/書き込み動作が検証されるにつれて、今後のリリースでセキュリティコンテキスト制約はさらに厳格化される予定です。

> [!note] LinuxパッケージインストールからCloud Native GitLabに移行する管理者は、`sudo`で実行されるLinuxパッケージインストールタスクが、OpenShiftおよび基盤となるKubernetesエンジンによって処理されることに注意してください。ポッドは個別のサービスであり、Linuxパッケージインストールでは、権限を落としてアプリケーション固有のユーザーとして実行されます。**Operator**は、[期待されるUIDで動作していないポッドをすべて終了](#security-context-constraints-within-the-gitlab-deployment)させます。

### サードパーティのリソース定義 {#third-party-resource-definitions}

### Ingressコントローラー {#ingress-controller}

GitLabは、クラウドネイティブGitLabをデプロイする際に、`nginx-ingress-controller`を使用したデプロイを推奨し、テストを行っています。これは独自の[`nginx-ingress-scc`セキュリティコンテキスト制約](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml)を使用します。

代替のIngressコントローラーを選択する場合は、関連ドキュメントを参照して、そのセキュリティコンテキスト制約の詳細を確認してください。

### SSL暗号化 {#ssl-encryption}

GitLab Operatorは、前提条件として[`cert-manager`](https://cert-manager.io/docs/releases/)を別途インストールする必要があります。GitLab Operatorは、`cert-manager` IssuerおよびCertificateを設定して、GitLabアプリケーション全体のTLSを管理します。`cert-manager`はセキュリティコンテキスト制約を直接設定しないため、OpenShiftはデフォルトで**restricted**セキュリティコンテキスト制約を適用します。
