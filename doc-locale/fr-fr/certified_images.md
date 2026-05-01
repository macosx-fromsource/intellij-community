---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Images certifiées RedHat
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

Le tableau suivant répertorie les images déployées par l'opérateur GitLab. Il inclut des liens vers les fiches de projet du portail technologique RedHat, où ces images peuvent être gérées par les membres de l'équipe GitLab.

Les tags d'image de l'opérateur GitLab sont alignés sur les
[versions de release de l'opérateur GitLab](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases).

Les tags d'image du contrôleur NGINX Ingress sont alignés sur le contenu du
[fichier `TAG`](https://gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx/-/blob/main/TAG) dans le
[fork du projet](https://gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx) géré par GitLab.

Les autres tags d'image suivent le format `v<version GitLab>-ubi8`, par exemple `v15.4.0-ubi8`. Le suffixe du tag indique
que les images ont été construites sur la base de la
[RedHat Universal Base Image (UBI)](https://catalog.redhat.com/software/containers/ubi8/ubi/5c359854d70cc534b3a3784e?container-tabs=overview),
une exigence pour la certification par RedHat. L'image de l'opérateur GitLab elle-même ne possède qu'une seule variante, déjà
construite sur la base d'UBI.

Consultez la [documentation Charts sur les images UBI](https://docs.gitlab.com/charts/advanced/ubi/)
pour plus d'informations, notamment des exemples de valeurs Helm permettant d'utiliser ces images.

| Composant                                                                                             | Chemin du registre |
|-------------------------------------------------------------------------------------------------------|---------------|
| [`gitlab-operator`](https://connect.redhat.com/component/629f9d952cb3e76438a9d40e/overview)           | `registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator:$OPERATOR_VERSION` |
| [`gitlab-operator-bundle`](https://connect.redhat.com/component/5f6cbaa04fcb1bc3f0425fbf/overview)    | `registry.connect.redhat.com/gitlab/gitlab-operator-bundle` |
| [`alpine-certificates`](https://connect.redhat.com/component/5fb615212977e7063dba93d0/overview)       | `registry.gitlab.com/gitlab-org/build/cng/alpine-certificates:$GITLAB_VERSION-ubi` |
| [`cfssl-self-sign`](https://connect.redhat.com/component/63474d9a673c3c4e34995d26/overview)           | `registry.gitlab.com/gitlab-org/build/cng/cfssl-self-sign:$GITLAB_VERSION-ubi` |
| [`kubectl`](https://connect.redhat.com/component/5fb611335e09a3c40183e67f/overview)                   | `registry.gitlab.com/gitlab-org/build/cng/kubectl:$GITLAB_VERSION-ubi` |
| [`gitaly`](https://connect.redhat.com/component/5fb60ec6c65ee7c76a2ad0d8/overview)                    | `registry.gitlab.com/gitlab-org/build/cng/gitaly:$GITLAB_VERSION-ubi` |
| [`gitlab-container-registry`](https://connect.redhat.com/component/5fb611e7935e0609ade7b2cb/overview) | `registry.gitlab.com/gitlab-org/build/cng/gitlab-container-registry:$GITLAB_VERSION-ubi` |
| [`gitlab-exporter`](https://connect.redhat.com/component/5fb60d575e09a3c40183e67c/overview)           | `registry.gitlab.com/gitlab-org/build/cng/gitlab-exporter:$GITLAB_VERSION-ubi` |
| [`gitlab-geo-logcursor`](https://connect.redhat.com/component/630683e0290892d1ec194033/overview)      | `registry.gitlab.com/gitlab-org/build/cng/gitlab-geo-logcursor:$GITLAB_VERSION-ubi` |
| [`gitlab-kas`](https://connect.redhat.com/component/6306824c0d53878b3b4cc60d/overview)                | `registry.gitlab.com/gitlab-org/build/cng/gitlab-kas:$GITLAB_VERSION-ubi` |
| [`gitlab-mailroom`](https://connect.redhat.com/component/5fb60e0a5e09a3c40183e67d/overview)           | `registry.gitlab.com/gitlab-org/build/cng/gitlab-mailroom:$GITLAB_VERSION-ubi` |
| [`gitlab-pages`](https://connect.redhat.com/component/630683acb2ab2de150584661/overview)              | `registry.gitlab.com/gitlab-org/build/cng/gitlab-pages:$GITLAB_VERSION-ubi` |
| [`gitlab-shell`](https://connect.redhat.com/component/5fb57b5d3379deb31cba93e5/overview)              | `registry.gitlab.com/gitlab-org/build/cng/gitlab-shell:$GITLAB_VERSION-ubi` |
| [`gitlab-sidekiq-ee`](https://connect.redhat.com/component/5fb60b4a2977e7063dba93cb/overview)         | `registry.gitlab.com/gitlab-org/build/cng/gitlab-sidekiq-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-toolbox-ee`](https://connect.redhat.com/component/60fb728cc3450afa1bb969e2/overview)         | `registry.gitlab.com/gitlab-org/build/cng/gitlab-toolbox-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-webservice-ee`](https://connect.redhat.com/component/5fb607e4c65ee7c76a2ad0d4/overview)      | `registry.gitlab.com/gitlab-org/build/cng/gitlab-webservice-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-workhorse-ee`](https://connect.redhat.com/component/5fb60c7b5e09a3c40183e67b/overview)       | `registry.gitlab.com/gitlab-org/build/cng/gitlab-workhorse-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-ingress-nginx`](https://connect.redhat.com/component/5fb60d575e09a3c40183e67c/overview)      | `registry.gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx/controller:$NGINX_VERSION-ubi` |

## Signatures des images {#image-signatures}

L'image de l'opérateur peut être vérifiée avec cosign :

```script
cosign verify "registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator:$OPERATOR_VERSION" \
  --certificate-identity "https://gitlab.com/gitlab-org/cloud-native/gitlab-operator//.gitlab-ci.yml@refs/heads/$OPERATOR_VERSION" \
  --certificate-oidc-issuer "https://gitlab.com"
```

Des informations sur la vérification de la signature des images CNG sont disponibles [dans la documentation du chart Helm](https://docs.gitlab.com/charts/installation/verify_cng_images/).
