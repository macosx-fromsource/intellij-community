---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: SSH経由でのGitのサポート
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

このドキュメントでは、さまざまな環境/プラットフォームにおけるSSH経由のGitの設定ガイドラインを提供します。

## 概要 {#overview}

[GitLab Shell Helmチャート](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/)は、GitLabへのGit SSHアクセス用に設定されたSSHサーバーを提供します。このコンポーネントは、ポート`22`でクラスターの外部に公開する必要があります。

`gitlab.gitlab-shell.enabled`が`true`に設定されている場合、GitLab Operatorは`gitlab-shell`をデプロイします。これはデフォルト設定です。

ターゲットプラットフォームに基づく要件の概要は次のとおりです。

| SSH経由のGitを必要とするか | Kubernetes                                                                                                    | OpenShift |
|------------------------------|---------------------------------------------------------------------------------------------------------------|-----------|
| いいえ                           | 以下のいずれかのNGINX Ingressプロバイダーを使用する必要があります（Kubernetesには組み込みのIngressプロバイダーがありません）。 | 以下のIngressプロバイダーは必要ありません。組み込みのRoutesをIngressプロバイダーとして使用できます。 |
| はい                          | 以下のいずれかのNGINX Ingressプロバイダーを使用する必要があります（Kubernetesには組み込みのIngressプロバイダーがありません）。 | 以下のいずれかのIngressプロバイダーを使用する必要があります。Routesはポート`22`の公開をサポートしていません。 |

## Ingressプロバイダー {#ingress-providers}

以下は、Ingressプロバイダーのリストと、関連するノートおよびプラットフォーム固有の詳細です。

### NGINX-Ingress Helmチャート {#nginx-ingress-helm-chart}

GitLabでは、[フォークした`NGINX-ingress`チャート](https://docs.gitlab.com/charts/charts/nginx/fork/)を管理しており、これを使用することで、SSH経由のGitを「すぐに」サポートするように変更されたNGINXリソースをデプロイできます。

これはGitLab Operatorを使用する場合のデフォルト設定であり、GitLab CR内の`nginx-ingress.enabled={true,false}`によって制御されます。`false`に設定すると、[外部NGINXインスタンス](https://docs.gitlab.com/charts/advanced/external-nginx/)を使用できます。

このIngressプロバイダーは、KubernetesとOpenShiftの両方で使用できます。

NGINX Ingressプロバイダーのインストールオプションの詳細については、[インストールに関するドキュメント](installation.md#ingress-controller)を参照してください。

### NGINX Ingress Operator {#nginx-ingress-operator}

組み込みのフォークしたNGINX-Ingress Helmチャートの代替として、[NGINX Ingress Operator](https://github.com/nginxinc/nginx-ingress-operator)を使用して`gitlab-shell`を公開することもできます。

このオプションにはいくつかの注意事項があります。

- NGINX Inc.のTransportServer/GlobalConfigurationカスタムリソース定義は、機能プレビューと見なされており、本番環境での使用には注意が必要です。
- NGINX Inc.Operatorはまだ比較的新しく、現在のバージョンは0.3.0に過ぎません。どちらのフレーバーの成熟したHelmチャートと比べても、利用可能な設定オプションはそれほど多く含まれていません。
- このオプションを使用する場合でも、NGINXサービスでポート`22`を_手動で_公開する必要があります（これは、NGINXIngressController CRでは設定できません）。

より広範な調査内容は、[\#58](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/58#note_585883916)に記載されています。

### OpenShift Routes {#openshift-routes}

OpenShift [Routes](https://docs.openshift.com/container-platform/3.4/architecture/core_concepts/routes.html)は、OpenShiftクラスターに組み込まれているコンポーネントです。OpenShiftにおけるRoutesは、[KubernetesにおけるIngress](https://kubernetes.io/docs/concepts/services-networking/ingress/)に相当します。

OpenShiftにデプロイする場合、GitLab CRで`nginx-ingress.enabled=false`を設定することで、外部トラフィックのフローをOpenShift Routesに制御させることができます。GitLab OperatorがIngressオブジェクトを調整すると、OpenShiftは、クラスターのベースドメインにマップする同等のRouteオブジェクトを自動的に作成します。

OpenShift RoutesはTCPトラフィック（ポート`22`のSSH）の公開をサポートしていないため、`gitlab-shell`を使用したSSH経由のGitには使用できません。

## 考慮事項 {#considerations}

以下は、Ingressを使用する際の考慮事項です。

### OpenShiftでサードパーティのIngressプロバイダーを使用する {#using-a-third-party-ingress-provider-in-openshift}

OpenShiftでサードパーティのIngressコントローラーを使用する場合、OpenShiftのIngressコントローラーがサードパーティのIngressコントローラーと競合することがあります。

たとえば、NGINX Ingressコントローラーは、Ingressの`ADDRESS`にNGINXサービスの外部IPアドレスを設定しますが、その後、OpenShift Ingressコントローラーがその設定をクラスターのベースドメインでオーバーライドします。これは、特に[external-dns](https://github.com/kubernetes-sigs/external-dns)のように、IngressにIPアドレスがあることを前提に、URLを特定のNGINX ServiceにマップするAレコードを作成するサービスを使用している場合、DNS設定と競合する可能性があります。これはまさに、GitLab Operator CI環境で発生し得る事象です。

この問題を回避するために、[OpenShift Ingressコントローラーにパッチを適用](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26)し、OpenShift固有のネームスペースのみを管理するようにしています。これにより、GitLab固有のネームスペースに作成するIngressが意図せず変更されることを防ぎます。
