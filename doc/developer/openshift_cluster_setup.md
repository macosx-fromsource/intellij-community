---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# OpenShift Cluster Setup

This document walks you through using the automation scripts in this project to create an OpenShift cluster in Google Cloud.

## Preparation

First, you should have a Red Hat account associated with your GitLab email.
Contact our Red Hat Alliance liaison; they will arrange to send you an account invitation email. After you activate your Red Hat account, you will have access to the licenses and subscriptions needed to run OpenShift.

To launch a cluster in Google Cloud, a public Cloud DNS zone must be connected to a registered domain and configured in Google Cloud DNS. If a domain is not already available, follow the steps [in this guide](https://github.com/openshift/installer/blob/master/docs/user/gcp/dns.md) to create one.

### Get the CLI tools and Pull Secret

Two CLI tools are required to create an OpenShift cluster (`openshift-install`) and then interact with the cluster (`oc`).

A pull secret is required to fetch images from Red Hat's private Docker registry.
Every developer has a different pull secret associated with their Red Hat account.

To get the CLI tools and your pull secret, go to <https://console.redhat.com/openshift/install/gcp/installer-provisioned> and log in with your Red Hat account.
On this page, download the latest version of the installer and command-line tools with the links provided. Extract these packages and place `openshift-install` and `oc` in your `PATH`.

Copy the pull secret to your clipboard and write the content to a file `pull_secret` in the root of this repository. This file is gitignored.

### Create a Google Cloud (GCP) Service Account

Follow [these instructions](https://docs.openshift.com/container-platform/4.15/installing/installing_gcp/installing-gcp-account.html) to create a Service Account in the Google Cloud `cloud-native` project. Attach all roles marked as Required in that document.
Once the Service Account is created, generate a JSON key and save it as `gcloud.json` in the root of this repository. This file is gitignored.

## Create your OpenShift cluster

Check [configuration options below](#configuration-options) and ensure that [required API services](https://docs.openshift.com/container-platform/4.15/installing/installing_gcp/installing-gcp-account.html#installation-gcp-enabling-api-services_installing-gcp-account) are enabled in the target GCP project.
Run `./scripts/create_openshift_cluster.sh` to create your OpenShift cluster in Google Cloud.
This will be a 6 node cluster with 3 control plane (master) nodes and 3 worker nodes ([configuration template](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/scripts/install-config.template.yaml)). This takes around 40 minutes. Follow the instructions at the end of the console output to connect to the cluster.

Once created, you should be able to see your cluster registered here: <https://console.redhat.com/openshift/>. All installation logs and metadata will be stored in the `install-$CLUSTER_NAME/` directory in this repository. This folder is gitignored.

If this cluster is meant to be used by other teammates or CI, create a new item in the 1Password Cloud Native vault and attach these files placed in `INSTALL_DIR`:

- `metadata.json`, delete clusters as needed
- `auth/kubeconfig`, authenticate to cluster
- `auth/kubeadmin-password`, authenticate to cluster UI

### Configuration options

Configuration can be applied during runtime by setting environment variables. All options have defaults, no options are required.

| Variable              | Description                                               | Default |
|-----------------------|-----------------------------------------------------------|---------|
| `CLUSTER_NAME`        | Name of cluster                                           | `ocp-$USER` |
| `BASE_DOMAIN`         | Root domain for cluster                                   | `k8s-ft.win` |
| `FIPS_ENABLED`        | Enable FIPS cryptography modules                          | `false` |
| `GCP_PROJECT_ID`      | Google Cloud project ID                                   | `cloud-native-182609` |
| `GCP_REGION`          | Google Cloud region for cluster                           | `us-central1` |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to Google Cloud service account JSON file | `gcloud.json` |
| `GOOGLE_CREDENTIALS`  | Content of Google Cloud service account JSON file         | Content of `$GOOGLE_APPLICATION_CREDENTIALS` |
| `PULL_SECRET_FILE`    | Path to Red Hat pull secret file                          | `pull_secret` |
| `PULL_SECRET`         | Content of Red Hat pull secret file                       | Content of `$PULL_SECRET_FILE` |
| `SSH_PUBLIC_KEY_FILE` | Path to SSH public key file                               | `$HOME/.ssh/id_rsa.pub` |
| `SSH_PUBLIC_KEY`      | Content of SSH public key file                            | Content of `$SSH_PUBLIC_KEY_FILE` |
| `LOG_LEVEL`           | Verbosity of `openshift-install` output                   | `info` |

NOTE:
The variables `CLUSTER_NAME` and `BASE_DOMAIN` are combined to build the domain name for the cluster.

NOTE:
Creating a cluster with `FIPS_ENABLED` set to `true` may cause issues with third party software.
We are investigating this in this issue: `https://gitlab.com/gitlab-org/charts/gitlab/-/issues/3153`.

## Destroy your OpenShift cluster

Run `./scripts/destroy_openshift_cluster.sh` to destroy your OpenShift cluster in Google Cloud. This takes around 4 minutes.

The `metadata.json` file in `INSTALL_DIR` is all that is needed to destroy an OpenShift cluster. `metadata.json` files are attached to the cluster's existing 1Password item that holds the cluster's credentials.

### Configuration options

Configuration can be applied during runtime by setting the following environment variables. All options have defaults, no options are required.

| Variable | Description | Default |
| - | - | - |
| `CLUSTER_NAME` | Name of cluster | `ocp-$USER` |
| `LOG_LEVEL` | Verbosity of `openshift-install` output | `info` |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to Google Cloud service account JSON file | `gcloud.json` |
| `GOOGLE_CREDENTIALS` | Content of Google Cloud service account JSON file | Content of `$GOOGLE_APPLICATION_CREDENTIALS` |

## Next Steps

See [doc/installation.md](installation.md) for instruction on installing the GitLab Operator in your OpenShift cluster.

## Resources

- `openshift-installer` source: <https://github.com/openshift/installer>
- `oc` source: <https://github.com/openshift/oc>
- `openshift-installer` and `oc` packages: <https://mirror.openshift.com/pub/openshift-v4/clients/ocp/>
- OpenShift Container Project (OCP) architecture documentation: <https://docs.openshift.com/container-platform/4.10/architecture/index.html>
- OpenShift GCP documentation: <https://docs.openshift.com/container-platform/4.15/installing/installing_gcp/installing-gcp-account.html>
- OpenShift troubleshooting guide: <https://docs.openshift.com/container-platform/4.15/support/troubleshooting/troubleshooting-installations.html>
