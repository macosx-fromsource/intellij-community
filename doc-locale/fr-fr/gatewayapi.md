---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Utiliser Gateway API et Envoy Gateway
---

{{< details >}}

- Édition : version gratuite, GitLab Premium, GitLab Ultimate
- Offre : GitLab Self-Managed
- Statut : version bêta

{{< /details >}}

> [!warning]
> Avant d'activer Gateway API avec l'opérateur GitLab, consultez la [documentation de Gateway API](https://docs.gitlab.com/charts/advanced/gateway-api/) du chart GitLab pour plus de détails sur les options de configuration disponibles et les limitations actuelles.

À partir de l'opérateur GitLab 2.10 et du chart GitLab 9.7, GitLab peut être exposé en utilisant la [passerelle API](https://gateway-api.sigs.k8s.io/) à la place d'Ingress. Cela suit la recommandation de la communauté Kubernetes suite à l'[abandon de NGINX Ingress](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/).

## Prérequis {#prerequisites}

L'opérateur GitLab n'intègre pas de contrôleur de passerelle API comme le fait le chart GitLab. Avant d'exposer une instance GitLab gérée par l'opérateur GitLab via Gateway API, vous devez d'abord installer une implémentation de Gateway API telle qu'[Envoy Gateway](https://gateway.envoyproxy.io/).

Si vous souhaitez utiliser Envoy Gateway et que vous utilisez le chart Helm Envoy Gateway officiel, assurez-vous que la prise en charge des EnvoyPatchPolicies est activée en définissant `config.envoyGateway.extensionsApi.enableEnvoyPatchPolicy=true` dans vos valeurs Envoy Gateway.

Si vous avez l'intention de gérer les certificats TLS avec certmanager, veillez à le configurer pour [Gateway API](https://cert-manager.io/docs/usage/gateway/).
