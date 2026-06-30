---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Gateway APIとEnvoy Gatewayを使用する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed
- ステータス: ベータ版

{{< /details >}}

> [!warning]
> OperatorでGateway APIを有効にする前に、利用可能な設定オプションと現在の制限事項の詳細について、GitLabチャートの[Gateway APIドキュメント](https://docs.gitlab.com/charts/advanced/gateway-api/)を確認してください。

Operator 2.10とGitLabチャート9.7以降、GitLabはIngressの代わりに[Gateway API](https://gateway-api.sigs.k8s.io/)を使用して公開できます。これは、[NGINX Ingressの提供終了](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/)後のKubernetesコミュニティの推奨事項に従うものです。

## 前提条件 {#prerequisites}

GitLab Operatorは、GitLabチャートのようにGateway APIコントローラーをバンドルしません。Operatorによって管理されるGitLabインスタンスをGateway APIを通じて公開する前に、まず[Envoy Gateway](https://gateway.envoyproxy.io/)などのGateway API実装をインストールする必要があります。

Envoy Gatewayを使用し、公式のEnvoy Gateway Helmチャートを使用する場合は、EnvoyPatchPoliciesのサポートが`config.envoyGateway.extensionsApi.enableEnvoyPatchPolicy=true`をEnvoy Gateway値に設定することで有効になっていることを確認してください。

certmanagerでTLS証明書を管理する予定がある場合は、[Gateway API](https://cert-manager.io/docs/usage/gateway/)用に設定されていることを確認してください。
