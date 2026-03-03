---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: セキュリティコンテキストの制約
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

## 概要 {#overview}

OpenShiftのポッドは、そのセキュリティコンテキスト制約に基づいてアクセス許可を受け取ります。セキュリティコンテキスト制約（多くの場合、**SCC**と略されます）により、大規模なデプロイで使用するためのロールベースのアクセス制御メカニズムが簡素化されます。[管理者は、アップストリームドキュメントを参照して、セキュリティコンテキスト制約の仕組みとOpenShiftでの役割についてより深く理解することができます](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html)

管理者は、次のリソースも参照できます:

1. [OpenShiftでのセキュリティコンテキスト制約の管理](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [OpenShiftとUIDのガイド](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## GitLabデプロイ内のセキュリティコンテキスト制約 {#security-context-constraints-within-the-gitlab-deployment}

`gitlab-controller-manager`デプロイは、**Operator**プロセスを含むポッドを作成および管理します。これと、それが作成および管理する他のポッドは、**restricted**セキュリティコンテキスト制約で実行されます。

**Operator**は、GitLabアプリケーションに必要なすべてのリソースを管理できるようにする、強力な権限を持つServiceAccountを使用します。

**Operator**は、Cloud Native GitLabを構成するコンポーネントサービスを管理します。これは、**Operator**によって指定されたUIDに準拠しないポッドを積極的に終了および置換します。このメカニズムは、最小特権の原則を適用します。

### GitLabアプリケーションのカスタムリソース定義 {#gitlab-application-custom-resource-definitions}

GitLabカスタムリソースを満たすためにオペレーターによってデプロイされたポッドは、**non-root-v2**セキュリティコンテキスト制約を使用します。サードパーティのオペレーターおよびリソースのセキュリティコンテキスト制約については、[次のセクションで説明します](#third-party-resource-definitions)。

`gitlab-app-nonroot` ServiceAccountには付与された権限がなく、GitLabアプリケーションポッドに**nonroot-v2**セキュリティコンテキスト制約をバインドするためだけに存在します。

OpenShiftセキュリティモデル内でGitLabアプリケーションの完全な読み取り/書き込み動作が検証されるため、セキュリティコンテキスト制約は将来のリリースで強化されます。

{{< alert type="note" >}}

LinuxパッケージインストールからCloud Native GitLabに移行する管理者は、`sudo`で実行されるLinuxパッケージインストールタスクは、OpenShiftと基盤となるKubernetesエンジンによって処理されることに注意してください。ポッドは個別のサービスであり、Linuxパッケージインストールでは、アプリケーション固有のユーザーとして実行するために特権を削除します。**Operator**は、[予期されるUIDで動作していないポッドをデプロイします](#security-context-constraints-within-the-gitlab-deployment)。

{{< /alert >}}

### サードパーティのリソース定義 {#third-party-resource-definitions}

### Ingressコントローラー {#ingress-controller}

GitLabは、Cloud Native GitLabをデプロイする際に、`nginx-ingress-controller`を使用したデプロイを推奨し、テストします。独自の[`nginx-ingress-scc`セキュリティコンテキスト制約](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml)を使用します。

代替のIngressコントローラーを選択する場合は、関連ドキュメントを参照して、そのセキュリティコンテキスト制約の詳細を確認してください。

### SSL暗号化 {#ssl-encryption}

**Operator**は、GitLabアプリケーション全体でSSL証明書を管理するために、[JetStackの**cert-manager-operator**](https://cert-manager.io/docs/releases/)をデプロイします。**cert-manager-operator**は、セキュアコンテキスト制約を直接設定しないため、OpenShiftはデフォルトで**restricted**セキュリティコンテキスト制約を適用します。
