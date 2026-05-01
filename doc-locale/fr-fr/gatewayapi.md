---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Utiliser Gateway API et Envoy Gateway
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed
- Status: Beta

{{< /details >}}

> [!warning]
> Avant d'activer Gateway API avec l'Operator, consultez la [documentation Gateway API](https://docs.gitlab.com/charts/charts/globals/#gateway-api) du chart GitLab
> pour obtenir des informations sur les options de configuration disponibles et les limitations actuelles.

À partir de l'Operator 2.10 et du chart GitLab 9.7, GitLab peut être exposé via [Gateway API](https://gateway-api.sigs.k8s.io/)
plutôt que via Ingress. Cette approche suit la recommandation de la communauté Kubernetes suite au
[retrait de NGINX Ingress](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/).

## Prérequis {#prerequisites}

L'Operator GitLab n'intègre pas de contrôleur Gateway API, contrairement au chart GitLab. Avant d'exposer
une instance GitLab gérée par l'Operator via Gateway API, vous devez d'abord installer une implémentation
Gateway API telle qu'[Envoy Gateway](https://gateway.envoyproxy.io/).

Si vous souhaitez utiliser Envoy Gateway avec le chart Helm officiel d'Envoy Gateway, assurez-vous que la prise en charge
des EnvoyPatchPolicies est activée en définissant `config.envoyGateway.extensionsApi.enableEnvoyPatchPolicy=true`
dans vos valeurs Envoy Gateway.

Si vous prévoyez de gérer les certificats TLS avec certmanager, veillez à le configurer pour
[Gateway API](https://cert-manager.io/docs/usage/gateway/).
