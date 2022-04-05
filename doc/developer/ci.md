# CI

## Review environments

The review environments will be automatically uninstalled after 1 hour. If you need the review environment to stay up longer, you can pin the environment
on the [Environments page](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/environments). However, make sure to manually trigger the jobs
in the `Cleanup` stage when you're done. This helps to ensure that the clusters have enough resources to run review apps for other merge requests.

See the [environments documentation](https://docs.gitlab.com/ee/ci/environments/index.html) for more information.

## OpenShift CI clusters

We manage OpenShift clusters in Google Cloud that are used for acceptance tests, including QA suite.

kubeconfig files for connecting to these clusters are stored in the 1Password cloud-native vault. Search for `ocp-ci`.

CI clusters have been launched with `scripts/create_openshift_cluster.sh` in this project. CI variables named `KUBECONFIG_OCP_4_7` allow scripts to connect to clusters as kubeadmin. `4_7` refers to the major and minor version of the targeted OpenShift cluster.

See [OpenShift cluster setup documentation](openshift_cluster_setup.md) for instruction on using this script.

### "public" and "dev" clusters

We maintain two sets of OpenShift CI clusters for this project:

- `public` CI clusters are responsible for everyday CI pipelines on [gitlab.com](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/pipelines).
- `dev` CI clusters are responsible for tagging and creating official releases on [dev.gitlab.org](https://dev.gitlab.org/gitlab/cloud-native/gitlab-operator/-/pipelines).

Every cluster is created using the [OpenShift cluster setup documentation](openshift_cluster_setup.md) and script, regardless of their set. For every OpenShift version deployed in public, there is a corresponding cluster with the same version deployed to dev. Both clusters are deployed to the `cloud-native` GCP project and use `k8s-ft.win` for DNS base domain.

### Scaling OpenShift clusters

OpenShift clusters use `machineset`s to pool resources. Default deployment has 4 `machineset`s out of which 2 are utilized.
We retain that approach and scale 2 active `machineset`s to 2 nodes.

Following [OpenShift scaling documentation](https://docs.openshift.com/container-platform/4.8/scalability_and_performance/recommended-cluster-scaling-practices.html) :

```shell
$ export KUBECONFIG=~/work/ocp/kubeconfig_4_8
$ oc get machinesets -n openshift-machine-api
NAME                         DESIRED   CURRENT   READY   AVAILABLE   AGE
ocp-ci-4717-abcde-worker-a   1         1         1       1           59d
ocp-ci-4717-abcde-worker-b   1         1         1       1           59d
ocp-ci-4717-abcde-worker-c   0         0                             59d
ocp-ci-4717-abcde-worker-f   0         0                             59d
```

create temporary capacity to handle workloads while we're scaling down:

```shell
$ oc scale --replicas=1 machineset ocp-ci-4717-abcde-worker-c -n openshift-machine-api
machineset.machine.openshift.io/ocp-ci-4717-abcde-worker-c scaled
$ oc get machinesets -n openshift-machine-api
NAME                         DESIRED   CURRENT   READY   AVAILABLE   AGE
ocp-ci-4717-abcde-worker-a   1         1         1       1           59d
ocp-ci-4717-abcde-worker-b   1         1         1       1           59d
ocp-ci-4717-abcde-worker-c   1         1                             59d
ocp-ci-4717-abcde-worker-f   0         0                             59d
$ kubectl get nodes
NAME                                                              STATUS     ROLES    AGE   VERSION
ocp-ci-4717-abcde-master-0.c.cloud-native-123456.internal         Ready      master   59d   v1.20.0+2817867
ocp-ci-4717-abcde-master-1.c.cloud-native-123456.internal         Ready      master   59d   v1.20.0+2817867
ocp-ci-4717-abcde-master-2.c.cloud-native-123456.internal         Ready      master   59d   v1.20.0+2817867
ocp-ci-4717-abcde-worker-a-wz8ts.c.cloud-native-123456.internal   Ready      worker   59d   v1.20.0+2817867
ocp-ci-4717-abcde-worker-b-shnf4.c.cloud-native-123456.internal   Ready      worker   59d   v1.20.0+2817867
ocp-ci-4717-abcde-worker-c-5gq6r.c.cloud-native-123456.internal   NotReady   worker   15s   v1.20.0+2817867
```

Scale down one of the `machineset`s:

```shell
oc scale --replicas=0 machineset ocp-ci-4717-abcde-worker-a -n openshift-machine-api
```

Wait for scale down to complete:

```shell
$ oc scale --replicas=2 machineset ocp-ci-4717-abcde-worker-a -n openshift-machine-api
$ kubectl get nodes
NAME                                                              STATUS   ROLES    AGE     VERSION
ocp-ci-4717-abcde-master-0.c.cloud-native-123456.internal         Ready    master   59d     v1.20.0+2817867
ocp-ci-4717-abcde-master-1.c.cloud-native-123456.internal         Ready    master   59d     v1.20.0+2817867
ocp-ci-4717-abcde-master-2.c.cloud-native-123456.internal         Ready    master   59d     v1.20.0+2817867
ocp-ci-4717-abcde-worker-b-shnf4.c.cloud-native-123456.internal   Ready    worker   59d     v1.20.0+2817867
ocp-ci-4717-abcde-worker-c-5gq6r.c.cloud-native-123456.internal   Ready    worker   7m18s   v1.20.0+2817867
```

scale back up to needed number of nodes:

```shell
$ oc scale --replicas=2 machineset ocp-ci-4717-abcde-worker-a -n openshift-machine-api
machineset.machine.openshift.io/ocp-ci-4717-abcde-worker-a scaled
$ kubectl get nodes
NAME                                                              STATUS   ROLES    AGE   VERSION
ocp-ci-4717-abcde-master-0.c.cloud-native-123456.internal         Ready    master   59d   v1.20.0+2817867
ocp-ci-4717-abcde-master-1.c.cloud-native-123456.internal         Ready    master   59d   v1.20.0+2817867
ocp-ci-4717-abcde-master-2.c.cloud-native-123456.internal         Ready    master   59d   v1.20.0+2817867
ocp-ci-4717-abcde-worker-a-n7qvs.c.cloud-native-123456.internal   Ready    worker   72s   v1.20.0+2817867
ocp-ci-4717-abcde-worker-a-sqn77.c.cloud-native-123456.internal   Ready    worker   73s   v1.20.0+2817867
ocp-ci-4717-abcde-worker-b-shnf4.c.cloud-native-123456.internal   Ready    worker   59d   v1.20.0+2817867
ocp-ci-4717-abcde-worker-c-5gq6r.c.cloud-native-123456.internal   Ready    worker   11m   v1.20.0+2817867
```

### external-dns

[external-dns](https://github.com/kubernetes-sigs/external-dns) has been installed using the [Bitnami Helm Chart](https://github.com/bitnami/charts/tree/master/bitnami/external-dns) using [ci/scripts/install_external_dns.sh](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/ci/scripts/install_external_dns.sh). This tool creates DNS entries for the NGINX Ingress controller Services that are created as external-facing LoadBalancers, ensuring that our QA jobs can reach the instance for testing.

- For our `4.8` OpenShift Cluster: `CLUSTER_VERSION=4.8 ENVIRONMENT=openshift GOOGLE_APPLICATION_CREDENTIALS=gitlab-operator-ci-gcloud-externaldns.json ./ci/scripts/install_external_dns.sh`
- For our `4.9` OpenShift Cluster: `CLUSTER_VERSION=4.9 ENVIRONMENT=openshift GOOGLE_APPLICATION_CREDENTIALS=gitlab-operator-ci-gcloud-externaldns.json ./ci/scripts/install_external_dns.sh`

Note: `gitlab-operator-ci-gcloud-externaldns.json` is a file containing the credentials for the external-dns ServiceAccount created in GCP. You can find this credentials file in 1Password by searching for `externaldns` in the `Cloud Native` vault.

## Configuration

### Job timeouts

Note: timeouts for Jobs can be configured. If the timeout is reached, then the GitLab Controller will return an error that the Job could not be completed in time.

To configure these, modify the values under `spec.template.spec.containers[0].env` in
[`config/manager/manager.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/config/manager/manager.yaml).

## Kubernetes CI clusters

We manage Kubernetes clusters in Google Cloud using GKE. These clusters are used to run the same acceptance tests that run on the OpenShift CI clusters.

Clusters are created using `charts/gitlab`'s [gke_bootstrap_script.sh script](https://gitlab.com/gitlab-org/charts/gitlab/-/blob/master/scripts/gke_bootstrap_script.sh).

```shell
$ CLUSTER_NAME='gitlab-operator' \
  PROJECT='cloud-native-182609' \
  REGION='europe-west3' \
  ZONE_EXTENSION='a' \
  USE_STATIC_IP='false' \
  EXTRA_CREATE_ARGS='' \
  ./scripts/gke_bootstrap_script.sh up
```

`demo/.kube/config` is generated and can be used to connect to the cluster with `kubectl` or `k9s` for development.

We then connect the cluster endpoint IP address by creating a new Cloud DNS record set.

```shell
$ ENDPOINT="$(gcloud container clusters describe gitlab-operator --zone europe-west3-a --format='value(endpoint)')"

$ gcloud dns record-sets create gitlab-operator.k8s-ft.win. \
  --rrdatas=$ENDPOINT --type A --ttl 60 --zone k8s-ftw
```

Once the cluster is created and connected with DNS, we run the `./scripts/install_certmanager` script to set up Letsencrypt TLS certificate.

```shell
$ KUBECONFIG=demo/.kube/config \
  CLUSTER_NAME=gitlab-operator \
  BASE_DOMAIN=k8s-ft.win \
  GCP_PROJECT_ID=cloud-native-182609 \
  ./scripts/install_certmanager.sh 'kubernetes'
```

Once wildcard certificates have been issued for the cluster's domain, the cluster is ready to run tests.

For CI clusters, we create a service account in Google Cloud, following the steps in [Google Cloud's authentication docs](https://cloud.google.com/kubernetes-engine/docs/how-to/api-server-authentication#environments-without-gcloud). This allows us to generate CI variables `KUBECONFIG_GKE` and `GOOGLE_APPLICATION_CREDENTIALS` for this project. We have scripted this, see [scripts/create_gcloud_sa_kubeconfig.sh](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/scripts/create_gcloud_sa_kubeconfig.sh). These kubeconfigconfig files are saved in the 1Password cloud-native vault. Search the vault for `gitlab-operator`.

### Scaling Kubernetes clusters

We [use the gcloud CLI to add or remove worker nodes from a GKE cluster](https://cloud.google.com/kubernetes-engine/docs/how-to/resizing-a-cluster#gcloud). The Google Cloud web UI may also be used.

For example, scaling the `gitlab-operator` CI cluster to 4 nodes:

```shell
$ gcloud container clusters resize \
  gitlab-operator --num-nodes 4
```
