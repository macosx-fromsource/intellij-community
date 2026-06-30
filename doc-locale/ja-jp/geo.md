---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab OperatorとGitLab Geoを設定する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

Operatorの要件、制限事項、およびGeoの設定は、[GitLabチャート](https://docs.gitlab.com/charts/advanced/geo/)の場合と同じです。

OperatorでGeoサイトをデプロイするには、Helmチャートの値を`spec.chart.values`に設定して、GitLabカスタムリソースに適用します。

## Gateway API {#gateway-api}

[Envoy GatewayでGateway APIを使用する](gatewayapi.md)場合、チャートのドキュメントに従う以外に追加の作業は不要です。Operatorは`global.geo.gatewayApi.additionalHostname`の設定を適用して、サイト間の内部通信を有効にします。

## Ingressクラス {#ingress-class}

> [!warning]
> NGINX IngressはGitLabチャート19.0で非推奨となり、GitLab 20.0で削除される予定です。
> 新しいGeoデプロイには[Envoy GatewayでGateway API](gatewayapi.md)を使用してください。
> 既存のGeoデプロイは、できるだけ早く移行してください。

GitLab Operatorには、セカンダリ[NGINX Ingress](https://docs.gitlab.com/charts/charts/nginx/#gitlab-geo)のIngressClassは付属していません。

このコントローラーとIngressClassが必要となるのは、次の場合のみです:

1. GitLab Geoに統合URLを使用する場合。
1. プライマリIngressコントローラーが受信`X-Forwarded-For`ヘッダーをオーバーライドする場合（バンドルされたデフォルトのNGINXチャートが該当します）。

IngressClassの作成プロセスは、インストール方法によって異なります:

{{< tabs >}}

{{< tab title="マニフェストとOLM" >}}

IngressClassは、デフォルトのマニフェストとOLMのリリースには含まれていません。

手動で作成します:

```shell
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: <gitlab-name>-nginx-geo
spec:
  controller: k8s.io/ingress-nginx-geo
EOF
```

{{< /tab >}}

{{< tab title="Helmチャート" >}}

値を更新して、IngressClassを有効にします:

```yaml
nginx-ingress:
  geo:
    ingressClass:
      enabled: true
```

{{< /tab >}}

{{< /tabs >}}
