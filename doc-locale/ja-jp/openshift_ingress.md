---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: OpenShiftのIngress
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab Self-Managed

{{< /details >}}

GitLab OperatorでOpenShiftにIngressを供給するために、2つのサポートされている方法があります:

- [NGINX Ingress Controller](#nginx-ingress-controller)
- [OpenShift Routes](#openshift-routes)

## NGINX Ingress Controller {#nginx-ingress-controller}

この構成では、トラフィックは次のように流れます:

```mermaid
graph TD
    U(End User) --> GTLB([gitlab.domain.com])
    GTLB -- resolves to --> SRV_N[/Service/gitlab-nginx-ingress-controller/]
    SRV_N -- connects to --> DPL_N[Deployment/gitlab-nginx-ingress-controller]
    DPL_N -- looks up corresponding ingress --> ING{{Ingress/gitlab-webservice-default}}
    ING -- proxies to --> SRV_W[/Service/gitlab-webservice-default/]
    SRV_W -- connects to --> DPL_W[Deployment/gitlab-webservice-default]
```

### OpenShift RouterがNGINX Ingress Controllerをオーバーライドする回避策 {#workaround-for-openshift-router-overriding-nginx-ingress-controller}

OpenShift環境では、GitLab Ingressは、NGINXサービスの外部IPアドレスではなく、GitLabインスタンスのホスト名を受信する場合があります。これは、`kubectl get ingress -n <namespace>`の出力の`ADDRESS`列に表示されます。

OpenShift Routerコントローラーは、Ingressクラスが異なるため、Ingressリソースを無視する代わりに、誤って更新します。次のコマンドは、OpenShiftにデプロイされた標準Ingress以外のIngressを適切に無視するようにOpenShift Routerコントローラーに指示します:

```shell
  kubectl -n openshift-ingress-operator \
    patch ingresscontroller default \
    --type merge \
    -p '{"spec":{"namespaceSelector":{"matchLabels":{"openshift.io/cluster-monitoring":"true"}}}}'
```

このパッチがIngressの作成後に適用された場合は、手動でIngressを削除してください。GitLab Operatorはそれらを手動で再作成します。これらは、NGINX Ingress Controllerによって適切に所有され、OpenShift Routerによって無視されるはずです。

{{< alert type="note" >}}

Ingressを手動で削除すると、バグが発生する可能性があります。回避策は、GitLab Operatorコントローラーポッドを手動で削除することです。詳細については、[\#315](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/315)を参照してください。

{{< /alert >}}

NGINX Ingress Controllerの作成をブロックしているSCC関連のイシューのトラブルシューティングについては、[Operatorトラブルシューティングドキュメント](troubleshooting.md#openshift-specific-problems)の追加ドキュメントを参照してください。

### 設定 {#configuration}

デフォルトでは、GitLab OperatorはGitLabの[NGINX Ingress Controllerチャートのフォーク](https://docs.gitlab.com/charts/charts/nginx/fork/)をデプロイします。

IngressにNGINX Ingress Controllerを使用するには、次の手順を実行します:

1. まず、[インストール手順](installation.md)の最初の手順に従って、GitLab Operatorをインストールします。
1. Webservice用に作成されたルートに関連付けられているドメイン名を見つけます:

   ```plaintext
   $ kubectl get route -n openshift-console console -ojsonpath='{.status.ingress[0].host}'

   console-openshift-console.yourdomain.com
   ```

   次の手順で使用するドメインは、`console-openshift-console`の_後_の部分です。

1. GitLab CRマニフェストが作成される手順で、ドメインを次のように設定します:

   ```yaml
   spec:
     chart:
       values:
         global:
           # Configure the domain from the previous step.
           hosts:
             domain: yourdomain.com
   ```

   {{< alert type="note" >}}

   デフォルトでは、CertManagerはGitLab関連のIngressのTLS証明書を作成および管理します。その他のオプションについては、[TLSドキュメント](https://docs.gitlab.com/charts/installation/tls/)を参照してください。

   {{< /alert >}}

1. 残りのインストール手順に従い、GitLab CRを適用し、CRステータスが最終的に`Ready`であることを確認します。
1. NGINX Ingress Controllerのサービス（LoadBalancerタイプ）の外部IPアドレスを見つけます:

   ```plaintext
   $ kubectl get svc -n gitlab-system gitlab-nginx-ingress-controller -ojsonpath='{.status.loadBalancer.ingress[].ip}'

   11.22.33.444
   ```

1. DNSクラウドプロバイダーでAレコードを作成して、ドメインと前の手順からの外部IPアドレスを接続します:

   - `gitlab.yourdomain.com` -> `11.22.33.444`
   - `registry.yourdomain.com` -> `11.22.33.444`
   - `minio.yourdomain.com` -> `11.22.33.444`

   ワイルドカードAレコードではなく個別のAレコードを作成すると、既存のルート（OpenShiftダッシュボードのルートなど）が期待どおりに動作し続けるようになります。

   {{< alert type="note" >}}

   これらのレコードは、クラウドプロバイダーのネットワーキング設定のパブリックゾーン_と_プライベートゾーン**と**に存在する必要があります。これらのゾーン間の同等性により、適切なクラスター内部ルーティングが保証され、CertManagerが証明書を適切に発行できるようになります。

   {{< /alert >}}

GitLabは`https://gitlab.yourdomain.com`で利用可能になるはずです。

## OpenShift Routes {#openshift-routes}

デフォルトでは、OpenShiftは[Routes](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html)を使用してIngressを管理します。

この構成では、トラフィックは次のように流れます:

```mermaid
graph TD
    U(End User) --> GTLB([gitlab.domain.com])
    GTLB -- resolves to --> SRV_R[/Service/router-default/]
    SRV_R -- connects to --> DPL_R[Deployment/router-default]
    DPL_R -- looks up corresponding Route --> RT{{Route/gitlab-webservice-default-xyz}}
    RT -- proxies to --> SRV_W[/Service/gitlab-webservice-default/]
    SRV_W -- connects to --> DPL_W[Deployment/gitlab-webservice-default]
```

{{< alert type="note" >}}

NGINX Ingress Controllerの代わりにルートをIngressに使用すると、[SSH](git_over_ssh.md)経由のGitはサポートされません。

{{< /alert >}}

### セットアップ {#setup}

IngressにOpenShiftルートを使用するには、次の手順を実行します:

1. まず、[インストール手順](installation.md)の最初の手順に従って、GitLab Operatorをインストールします。
1. Webservice用に作成されたルートに関連付けられているドメイン名を見つけます:

   ```plaintext
   $ kubectl get route -n openshift-console console -ojsonpath='{.status.ingress[0].host}'
   console-openshift-console.yourdomain.com
   ```

   次の手順で使用するドメインは、`console-openshift-console`の_後_の部分です。
1. GitLab CRマニフェストが作成される手順で、以下も設定します:

   ```yaml
   spec:
     chart:
       values:
         # Disable NGINX Ingress Controller.
         nginx-ingress:
           enabled: false
         global:
           # Configure the domain from the previous step.
           hosts:
             domain: yourdomain.com
           ingress:
             # Unset `spec.ingressClassName` on the Ingress objects
             # so the OpenShift Router takes ownership.
             class: none
             annotations:
               # The OpenShift documentation says "edge" is the default, but
               # the TLS configuration is only passed to the Route if this annotation
               # is manually set.
               route.openshift.io/termination: "edge"
   ```

   {{< alert type="note" >}}

   デフォルトでは、CertManagerはGitLab関連のルートのTLS証明書を作成および管理します。その他のオプションについては、[TLSドキュメント](https://docs.gitlab.com/charts/installation/tls/)を参照してください。OpenShiftクラスターがワイルドカード証明書で保護されている場合、[オプション2](https://docs.gitlab.com/charts/installation/tls/#option-2-use-your-own-wildcard-certificate)を使用すると、ワイルドカード証明書でGitLab関連のルートを保護できます。

   {{< /alert >}}

1. 残りのインストール手順に従い、GitLab CRを適用し、CRステータスが最終的に`Ready`であることを確認します。

GitLabは`https://gitlab.yourdomain.com`で利用可能になるはずです。

この構成では、OpenShiftルートは、GitLab Operatorによって作成されたIngressを変換することによって作成されます。この変換の詳細については、[ルートドキュメント](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html#nw-ingress-creating-a-route-via-an-ingress_route-configuration)を参照してください。
