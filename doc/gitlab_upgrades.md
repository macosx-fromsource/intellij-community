---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Upgrading GitLab
---

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab Self-Managed

The GitLab Operator is capable of managing upgrades between versions of GitLab. This document includes background context on how the upgrade flow works under the hood, along with instructions to perform a GitLab upgrade.

## How the Operator handles GitLab upgrades

At the beginning of the controller reconcile loop, the Operator checks if the current version matches the desired version.

- If these versions match, then the regular reconcile loop executes, ensuring objects exist that satisfy the configuration provided in the CR spec.
- If these versions _do not_ match, the regular reconcile loop still executes, but an additional branch of logic executes to handle the upgrade flow.

The upgrade flow behaves like this:

1. The controller reconciles all Deployments.
   - The Webservice and Sidekiq Deployments are reconciled but are "paused". This means that the "old" pods stay up until the new Deployments are unpaused.
1. Pre-migrations run.
   - This effectively just runs the Migrations job, but skips post-deployment migrations.
1. The controller unpauses the Webservice and Sidekiq Deployments.
1. The controller waits for the new Webservice and Sidekiq pods to be running.
1. Post-migrations run.
   - This runs the Migrations job (without skipping post-deployment migrations).
1. The controller performs a rolling update on the Webservice and Sidekiq Deployments.
1. The controller waits for the restarted Webservice and Sidekiq pods to be running.

In future reconcile loops, this branch of logic is skipped because the desired version (from `spec.chart.version`) matches the current version (from `status.version`).

## How to upgrade GitLab

Below are the steps to upgrade a GitLab instance using the GitLab Operator.

### Step 1

Update your GitLab CR's `spec.chart.version` field to a new version. For example:

```diff
apiVersion: apps.gitlab.com/v1beta1
kind: GitLab
metadata:
  name: gitlab
spec:
  chart:
-   version: "5.0.6"
+   version: "5.1.1"
    values:
      ...
```

### Step 2

Apply your modified GitLab CR to the cluster:

```shell
kubectl -n gitlab-system apply -f mygitlab.yaml
```

You should see the following message:

```shell
gitlab.apps.gitlab.com/gitlab created
```

### Step 3

You can watch the progress via the controller logs:

```shell
$ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
...
```

You see log entries following the upgrade flow outlined above.

You can also view the GitLab CR status in the cluster:

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS        VERSION
gitlab   Preparing     5.2.4
```

When the application is ready and upgraded to the new version, you see it reflected in the `STATUS` column.

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS      VERSION
gitlab   Running     5.2.4
```

Status conditions on the GitLab object itself present more detailed information about the application.

## Additional upgrade considerations

Below are additional topics to consider when before upgrading a GitLab instance.

- [Restoring data when PersistentVolumeClaim configuration changes](troubleshooting.md#restoring-data-when-persistentvolumeclaim-configuration-changes): This was particularly relevant in Operator 0.6.4, when [!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419) replaced the Operator-defined MinIO objects with MinIO objects from the GitLab Helm Charts.
