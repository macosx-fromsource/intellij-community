---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configurer l'opérateur GitLab avec GitLab Geo
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

Les exigences, les limitations et la configuration Geo de l'opérateur
sont identiques à celles du [chart GitLab](https://docs.gitlab.com/charts/advanced/geo/).

Pour déployer des sites Geo avec l'opérateur, appliquez les valeurs du chart Helm à
la ressource personnalisée GitLab en définissant `spec.chart.values`.

## Classe Ingress {#ingress-class}

L'opérateur GitLab n'est pas fourni avec une IngressClass pour le
[NGINX Ingress](https://docs.gitlab.com/charts/charts/nginx/#gitlab-geo) secondaire.

Ce contrôleur et cette IngressClass ne sont nécessaires que si :

1. Vous souhaitez utiliser une URL unifiée pour GitLab Geo.
1. Votre contrôleur Ingress principal remplace les en-têtes `X-Forwarded-For`
   entrants (ce que fait le chart NGINX par défaut inclus).

Le processus de création de l'IngressClass dépend de votre méthode d'installation :

{{< tabs >}}

{{< tab title="Manifest and OLM" >}}

L'IngressClass n'est pas incluse dans le manifest par défaut ni dans la release OLM.

Créez-la manuellement :

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

{{< tab title="Helm Chart" >}}

Activez l'IngressClass en mettant à jour vos valeurs :

```yaml
nginx-ingress:
  geo:
    ingressClass:
      enabled: true
```

{{< /tab >}}

{{< /tabs >}}
