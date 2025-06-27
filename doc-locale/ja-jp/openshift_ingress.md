---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: OpenShiftのIngress
---

{{< details >}}

- プラン:Free, Premium, Ultimateプラン
- 提供:GitLab Self-Managed

{{< /details >}}

GitLab OperatorでOpenShiftにIngressを提供するには、2つのサポートされている方法があります。

- [NGINX Ingress Controller](#nginx-ingress-controller)（デフォルト）
- [OpenShiftルート](#openshift-routes)

## NGINX Ingress Controller {#nginx-ingress-controller}

この構成では、トラフィックは次のようになります。

```mermaid
graph TD
    U(End User) --> GTLB([gitlab.domain.com])
    GTLB -- resolves to --> SRV_N[/Service/gitlab-nginx-ingress-controller/]
    SRV_N -- connects to --> DPL_N[Deployment/gitlab-nginx-ingress-controller]
    DPL_N -- looks up corresponding ingress --> ING{{Ingress/gitlab-webservice-default}}
    ING -- proxies to --> SRV_W[/Service/gitlab-webservice-default/]
    SRV_W -- connects to --> DPL_W[Deployment/gitlab-webservice-default]
```

### OpenShift RouterがNGINX Ingress Controllerをオーバーライドする場合の回避策 {#workaround-for-openshift-router-overriding-nginx-ingress-controller}

OpenShift環境では、GitLab Ingressは、NGINX Serviceの外部IPアドレスの代わりに、GitLabインスタンスのホスト名を受信する場合があります。これは、`kubectl get ingress -n <namespace>`の`ADDRESS`列の出力に表示されます。

OpenShift Routerコントローラーは、Ingressクラスが異なるため、Ingressリソースを無視する代わりに、誤って更新します。次のコマンドは、OpenShiftにデプロイされた標準のIngress以外のIngressを適切に無視するようにOpenShift Routerコントローラーに指示します。

```shell
  kubectl -n openshift-ingress-operator \
    patch ingresscontroller default \
    --type merge \
    -p '{"spec":{"namespaceSelector":{"matchLabels":{"openshift.io/cluster-monitoring":"true"}}}}'
```

Ingressの作成後にこのパッチを適用した場合は、Ingressを手動で削除してください。GitLab Operatorは、それらを手動で再作成します。それらは、NGINX Ingress Controllerによって適切に所有され、OpenShift Routerによって無視されるはずです。

{{< alert type="note" >}}

Ingressを手動で削除すると、バグが発生する可能性があります。回避策は、GitLab OperatorコントローラーのPodを手動で削除することです。詳細については、[\#315](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/315)を参照してください。

{{< /alert >}}

NGINX-Ingress [Controller](troubleshooting.md#openshift-specific-problems)の作成をブロックしているSCC関連の[イシュー](troubleshooting.md#openshift-specific-problems)については、[GitLab Operator](troubleshooting.md#openshift-specific-problems)の[トラブルシューティング](troubleshooting.md#openshift-specific-problems)ドキュメントにある追加のドキュメントを参照してください。

### 設定 {#configuration}

デフォルトでは、GitLab Operatorは、GitLabの[フォークしたNGINX Ingress Controllerをデプロイします。](https://docs.gitlab.com/charts/charts/nginx/fork/)

IngressにNGINX Ingress Controllerを使用するには、次の手順を実行します。

1. [GitLab Operator](installation.md)をインストールするには、[インストール手順](installation.md)の最初の手順に従って開始します。
1. 用に作成されたに関連付けられているドメイン名を見つけます。

   ```plaintext
   $ kubectl get route -n openshift-console console -ojsonpath='{.status.ingress[0].host}'

   console-openshift-console.yourdomain.com
   ```

   次のステップで使用するドメインは、`console-openshift-console` _の後の_部分です。

1. GitLab CRが作成されるステップで、次のようにドメインを設定します。

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

デフォルトでは、CertManagerは、関連ののTLS証明書を作成して管理します。詳細については、[TLSドキュメント](https://docs.gitlab.com/charts/installation/tls/)を参照してください。

   {{< /alert >}}

1. 残りのインストール手順に従ってGitLab CRを適用し、CRのstatusが最終的に`Ready`になることを確認します。
1. NGINX Ingress ControllerのService（LoadBalancerタイプ）の外部IPアドレスを見つけます。

   ```plaintext
   $ kubectl get svc -n gitlab-system gitlab-nginx-ingress-controller -ojsonpath='{.status.loadBalancer.ingress[].ip}'

   11.22.33.444
   ```

1. DNSプロバイダーでAレコードを作成して、ドメインと前の手順からの外部IPアドレスを接続します。

   - `gitlab.yourdomain.com` -&gt; `11.22.33.444`
   - `registry.yourdomain.com` -&gt; `11.22.33.444`
   - `minio.yourdomain.com` -&gt; `11.22.33.444`

   ワイルドカードAレコードではなく、個々のAレコードを作成すると、既存の（OpenShiftのなど）が期待どおりに動作し続けることが保証されます。

   {{< alert type="note" >}}

これらのレコードは、_クラウドプロバイダー_の**ネットワーキング** **設定**で、パブリックゾーン_と_プライベートゾーン_の両方_に存在する必要があります。これらのゾーン間の同等性により、適切な内部ルーティングが保証され、CertManagerが証明書を適切に発行できるようになります。

   {{< /alert >}}

これで、`https://gitlab.yourdomain.com`でが利用可能になります。

## OpenShiftルート {#openshift-routes}

デフォルトでは、OpenShiftは[ルート](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html)を使用してを管理します。

この構成では、トラフィックは次のようになります。

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

NGINX Ingress Controllerの代わりにIngressにルートを使用するということは、[SSH経由のGit](git_over_ssh.md)がサポートされていないことを意味します。

{{< /alert >}}

### セットアップ {#setup}

IngressにOpenShiftルートを使用するには、次の手順を実行します。

1. [GitLab Operator](installation.md)をインストールするには、[インストール手順](installation.md)の最初の手順に従って開始します。
1. 用に作成されたに関連付けられているドメイン名を見つけます。

   ```plaintext
   $ kubectl get route -n openshift-console console -ojsonpath='{.status.ingress[0].host}'
   console-openshift-console.yourdomain.com
   ```

   次のステップで使用するドメインは、`console-openshift-console` _の後の_部分です。
1. が作成されるステップで、次のようにも設定します。

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

デフォルトでは、CertManagerは関連ののTLS証明書を作成し、管理します。詳細については、[TLSドキュメント](https://docs.gitlab.com/charts/installation/tls/)を参照してください。OpenShiftクラスターがワイルドカード証明書で保護されている場合、[オプション2](https://docs.gitlab.com/charts/installation/tls/#option-2-use-your-own-wildcard-certificate)を使用すると、ワイルドカード証明書でGitLab関連のRouteを保護できます。

   {{< /alert >}}

1. 残りのインストール手順に従ってGitLab CRを適用し、CRのstatusが最終的に`Ready`になることを確認します。

これで、`https://gitlab.yourdomain.com`でが利用可能になります。

この構成では、OpenShiftルートは、GitLab Operatorによって作成されたIngressを変換することによって作成されます。この変換の詳細については、[Routeドキュメント](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html#nw-ingress-creating-a-route-via-an-ingress_route-configuration)を参照してください。
