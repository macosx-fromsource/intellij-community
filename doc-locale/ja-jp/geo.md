---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab OperatorをGitLab Geoと連携するように設定する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

Operatorの要件、制限事項、およびGeoの設定は、[GitLabチャート](https://docs.gitlab.com/charts/advanced/geo/)と同じです。

OperatorでGeoサイトをデプロイするには、`spec.chart.values`を設定して、Helmチャートの値をGitLabカスタムリソースに適用します。

## Ingressクラス {#ingress-class}

GitLab Operatorには、セカンダリ[NGINX Ingress](https://docs.gitlab.com/charts/charts/nginx/#gitlab-geo)のIngressClassは付属していません。

このコントローラーとIngressClassが必要となるのは、次の場合のみです:

1. GitLab Geoに統合URLを使用する場合。
1. プライマリIngressコントローラーが受信の`X-Forwarded-For`ヘッダーをオーバーライドする（バンドルされているデフォルトのNGINXチャートがオーバーライドする）

IngressClassの作成プロセスは、インストール方法によって異なります:

{{< tabs >}}

{{< tab title="マニフェストとOLM" >}}

IngressClassは、デフォルトのマニフェストとOLMのリリースには含まれていません。

手動で作成:

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
