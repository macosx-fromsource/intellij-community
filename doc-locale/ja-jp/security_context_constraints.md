---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: セキュリティコンテキスト制約
---

{{< details >}}

- プラン:Free, Premium, Ultimate
- 提供:GitLab Self-Managed

{{< /details >}}

## 概要 {#overview}

OpenShiftのポッドは、セキュリティコンテキスト制約に基づいて権限を受け取ります。セキュリティコンテキスト制約（多くの場合、_**SCC**_と略されます）は、大規模な_**SCC**_におけるロールベースのアクセス制御メカニズムの使用を簡素化します。[管理者は、アップストリームドキュメントを参照して、セキュリティコンテキスト制約の仕組みとOpenShiftでの役割について、より詳しくインサイトを得ることができます](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html)

管理者は、以下のリソースも参照できます。

1. [OpenShiftでのセキュリティコンテキスト制約の管理](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [OpenShiftとUIDのガイド](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## GitLabSCC内のセキュリティコンテキスト制約 {#security-context-constraints-within-the-gitlab-deployment}

この`gitlab-controller-manager`デプロイは、**Operator**プロセスを含むポッドを作成および管理します。これと、それが作成および管理する他のポッドは、_**restricted**_セキュリティコンテキスト制約で実行されます。

**Operator**は、GitLabアプリケーションに必要なすべてのリソースを管理できるようにする、堅牢な権限を持つServiceAccountを使用します。

**Operator**は、クラウドネイティブGitLabを構成するコンポーネントサービスを管理します。**Operator**によって指定された固有識別子（UID）に準拠しないポッドを積極的に終了および置き換えます。このメカニズムは、最小権限の原則を適用します。

### GitLabアプリケーションカスタムリソース定義 {#gitlab-application-custom-resource-definitions}

GitLabカスタムリソースを満たすためにOperatorによって_デプロイ_されたポッドは、_**non-root-v2**_セキュリティコンテキスト制約を使用します。サードパーティのOperatorおよびリソースのセキュリティコンテキスト制約については、[次のセクションで説明します](#third-party-resource-definitions)。

`gitlab-app-nonroot` ServiceAccountには、権限が付与されておらず、GitLabアプリケーションポッドに_**nonroot-v2**_セキュリティコンテキスト制約をバインドするためだけに存在します。

OpenShiftセキュリティモデル内でGitLabアプリケーションの完全な_読み取り/書き込み_動作が検証されると、セキュリティコンテキスト制約は将来の_リリース_で強化されます。

{{< alert type="note" >}}

LinuxパッケージインストールからクラウドネイティブGitLabに移行する管理者は、`sudo`で実行されるLinuxパッケージインストールタスクは、OpenShiftおよび基盤となるKubernetesエンジンによって処理されることに注意してください。ポッドは個別のサービスであり、Linuxパッケージインストールでは、アプリケーション固有のユーザーとして実行するための権限を削除します。**Operator**は、[予期される固有識別子（UID）で動作していないポッドを終了します](#security-context-constraints-within-the-gitlab-deployment)。

{{< /alert >}}

### サードパーティリソース定義 {#third-party-resource-definitions}

### Ingressコントローラー {#ingress-controller}

GitLabは、クラウドネイティブGitLabのデプロイ時に`nginx-ingress-controller`を使用したデプロイを推奨し、テストします。独自の[`nginx-ingress-scc`セキュリティコンテキスト制約](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml)を使用します。

代替のIngressコントローラーを選択する場合は、関連ドキュメントを参照して、セキュリティコンテキスト制約について詳しく見てください。

### SSL暗号化 {#ssl-encryption}

**Operator**は、GitLabアプリケーション全体でSSL証明書を管理するために、[JetStackからの**cert-manager-operator**](https://cert-manager.io/docs/releases/)を**デプロイ**します。**cert-manager-operator**は、セキュアコンテキスト制約を直接設定しないため、OpenShiftはデフォルトで_**restricted**_セキュリティコンテキスト制約を適用します。
