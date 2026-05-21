---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Configurer l'opérateur GitLab avec GitLab Geo"
---

{{< details >}}

- Édition :  version gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab Self-Managed

{{< /details >}}

Les exigences, les limitations et la configuration de GitLab Geo de l'opérateur GitLab sont les mêmes que pour le [chart GitLab](https://docs.gitlab.com/charts/advanced/geo/).

Pour déployer des sites GitLab Geo avec l'opérateur GitLab, appliquez les valeurs du chart Helm à la ressource personnalisée GitLab en définissant `spec.chart.values`.

## Classe Ingress {#ingress-class}

L'opérateur GitLab n'est pas fourni avec une IngressClass de [NGINX Ingress](https://docs.gitlab.com/charts/charts/nginx/#gitlab-geo) secondaire.

Ce contrôleur et cette IngressClass ne sont nécessaires que si :

1. Vous souhaitez utiliser une URL unifiée pour GitLab Geo.
1. Votre contrôleur Ingress principal remplace les en-têtes entrants `X-Forwarded-For` (ce que fait le chart NGINX intégré par défaut).

Le processus de création de l'IngressClass dépend de votre méthode d'installation :

{{< tabs >}}

{{< tab title="Manifest and OLM" >}}

L'IngressClass n'est pas incluse dans le manifeste par défaut et la release OLM.

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
