---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH経由でのGitのサポート
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

[GitLab Shell Helmチャート](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/)は、GitLabへのGit SSHアクセス用に設定されたSSHサーバーを提供します。このコンポーネントは、ポート`22`でクラスターの外部に公開する必要があります。

`gitlab.gitlab-shell.enabled`が`true`に設定されている場合、GitLab Operatorは`gitlab-shell`をデプロイします。これはデフォルト設定です。

以下のいずれかの方法を使用して、SSH経由のGitを公開します。

| 方法 | Kubernetes | OpenShift | 備考 |
|:-----------------|:-----------|:---------------|:------|
| Gateway API | サポート対象 | サポート対象 | TCPRouteを使用したKubernetes Gateway API標準に基づく推奨の最新アプローチ。Envoy Gatewayを推奨。[その他のプロバイダー](https://docs.gitlab.com/charts/advanced/gateway-api/#using-an-external-gateway-api-provider)は要件を満たす場合に使用可能。 |
| NGINX Ingress | 非推奨 | 非推奨 | 従来のアプローチ（ポート22の公開が必要）。バンドルされているものの代わりに[外部NGINXコントローラー](https://docs.gitlab.com/charts/advanced/external-ingress/)を使用可能。 |
| OpenShift Routes | N/A | SSHサポートなし | RoutesはTCPトラフィック（ポート22）をサポートしていません。 |

## Envoy GatewayによるGateway API {#gateway-api-with-envoy-gateway}

GitLabは、従来のIngressリソースの代わりに[Gateway API](https://gateway-api.sigs.k8s.io/)を使用して公開できます。この方法は、`TCPRoute`リソースを通じてSSH経由のGitのTCPルーティングをネイティブにサポートするため、新規デプロイに推奨されます。

前提条件:

- GitLab Operator 2.10以降。
- GitLabチャート9.7以降。

Gateway APIは、KubernetesとOpenShiftの両方のクラスターで動作します。詳細な設定手順と前提条件については、[Gateway APIとEnvoy Gatewayのドキュメント](gatewayapi.md)を参照してください。

`global.gatewayApi.enabled: true`でGateway APIを有効にすると、`gitlab-shell`はポート`22`のTCPトラフィックをGitLab ShellサービスにルーティングするTCPRouteリソースを通じて自動的に公開されます。

## NGINX Ingress {#nginx-ingress}

> [!warning]
> NGINX IngressはGitLabチャート19.0で非推奨となり、GitLab 20.0で削除される予定です。
> 新規デプロイには[Envoy GatewayによるGateway API](#gateway-api-with-envoy-gateway)を使用してください。

GitLab OperatorはKubernetesとOpenShiftの両方でNGINX Ingressコントローラーをサポートしています。NGINX Ingressを使用する場合、SSH経由のGitを有効にするには、NGINXサービスでポート`22`を公開する必要があります。

GitLabは、SSH経由のGitをサポートするように設定されたNGINXリソースをデプロイするために使用できる[フォークした`NGINX-ingress`チャート](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork)を管理しています。このチャートは非推奨でサポートされていませんが、既存のデプロイでは引き続き使用できます。

NGINX Ingressは、GitLab CR内の`nginx-ingress.enabled={true,false}`によって制御されます。`false`に設定すると、[外部NGINXインスタンス](https://docs.gitlab.com/charts/advanced/external-nginx/)を使用できます。

## OpenShift Routes {#openshift-routes}

OpenShift [Routes](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html)は、OpenShiftクラスターに組み込まれているIngressソリューションです。GitLab CRで`nginx-ingress.enabled=false`を設定してNGINX Ingressを無効にすると、OpenShiftはOperatorが作成したIngressオブジェクトを同等のRouteオブジェクトに自動的に変換します。

OpenShift RoutesはTCPトラフィック（ポート`22`）の公開をサポートしていないため、OpenShift Routesを使用する場合はSSH経由のGitはサポートされません。OpenShiftでSSH経由のGitが必要な場合は、次のいずれかを選択してください。

- Envoy GatewayによるGateway APIを使用する。GitLab CRで`global.gatewayApi.enabled=true`を設定します。
- NGINX Ingressコントローラーを使用する。`nginx-ingress.enabled=true`（デフォルト）を設定します。

OpenShiftのIngressオプションの詳細については、[OpenShiftにおけるIngress](openshift_ingress.md)を参照してください。

## 考慮事項 {#considerations}

以下は、Ingressを使用する際の考慮事項です。

### OpenShiftでサードパーティのIngressプロバイダーを使用する {#using-a-third-party-ingress-provider-in-openshift}

OpenShiftでサードパーティのIngressコントローラーを使用する場合、OpenShiftのIngressコントローラーがサードパーティのIngressコントローラーと競合することがあります。

たとえば、NGINX Ingressコントローラーは、Ingressの`ADDRESS`にNGINXサービスの外部IPアドレスを設定しますが、その後、OpenShift Ingressコントローラーがその設定をクラスターのベースドメインでオーバーライドします。これは、特に[external-dns](https://github.com/kubernetes-sigs/external-dns)のように、IngressにIPアドレスがあることを前提に、URLを特定のNGINX ServiceにマップするAレコードを作成するサービスを使用している場合、DNS設定と競合する可能性があります。これはまさに、GitLab Operator CI環境で発生し得る事象です。

この問題を回避するために、[OpenShift Ingressコントローラーにパッチを適用](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26)し、OpenShift固有のネームスペースのみを管理するようにしています。これにより、GitLab固有のネームスペースに作成するIngressが意図せず変更されることを防ぎます。
