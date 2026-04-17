---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Upgrade GitLab instances with Operator
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

You can use GitLab Operator to upgrade GitLab instances that were installed with GitLab Operator.

## Prerequisites

Before you upgrade with GitLab Operator:

1. Consult [information you need before you upgrade](https://docs.gitlab.com/update/plan_your_upgrade/).
1. Identify the version of GitLab Operator required for the version of GitLab you want. For mappings between
   GitLab versions, GitLab Helm chart versions, and GitLab Operator versions, see the GitLab Operator
   [releases](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases).

## Upgrade GitLab with GitLab Operator

To upgrade GitLab with GitLab Operator:

1. Consider [turning on maintenance mode](https://docs.gitlab.com/administration/maintenance_mode/) during the upgrade
   to restrict users from write operations to help not disturb any workflows.
1. [Upgrade GitLab Runner](https://docs.gitlab.com/runner/install/) to the same version as your target GitLab version.
1. [Upgrade GitLab Operator](#upgrade-gitlab-operator).
1. [Upgrade GitLab by using GitLab Operator](#upgrade-gitlab-by-using-gitlab-operator).

After you upgrade:

1. If enabled, [turn off maintenance mode](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode).
1. Run [upgrade health checks](https://docs.gitlab.com/update/plan_your_upgrade/#run-upgrade-health-checks).

### Upgrade GitLab Operator

To upgrade GitLab Operator:

1. Perform a [backup](https://docs.gitlab.com/charts/backup-restore/).
1. Install the required version by using `kubectl` to apply the manifest for the required version of GitLab Operator.

   ```shell
   VERSION=X.Y.Z
   kubectl apply -f \
     https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
   ```

   This command applies any changes to the related manifests, including the new deployment image to use.

1. Confirm that the new version of GitLab Operator becomes the leader. The GitLab Operator deployment should create a new
   ReplicaSet with this change, which spawns a new GitLab Operator pod. Meanwhile, the previous GitLab Operator pod
   shuts down, giving up its leader status. When this happens, the new GitLab Operator pod becomes the leader.
1. Update the chart version in the GitLab custom resource (CR). In most cases, the available chart versions is not
   identical between versions of GitLab Operator. When the newer version of GitLab Operator starts, it tries to
   reconcile the existing GitLab custom resource (CR). You might see an error such as:

   ```plaintext
   Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
   ```

   To address this, identify a valid version from that release's available chart versions. For example, when upgrading
   from Operator `0.4.0` to `0.4.1`, update the GitLab CR to an available chart version closest to `5.7.0`, which in this
   case is `5.7.1`.

1. Confirm that GitLab Operator reconciles GitLab as expected. Check the logs from the new operator pod to see if the operator pod
   upgraded to the defined chart version.

   To confirm that the upgrade was successful, get the status of the GitLab CR:

   ```plaintext
   $ kubectl get gitlabs -n gitlab-system
   NAME     STATUS    VERSION
   gitlab   Running   5.7.1
   ```

   The status `Running` means that GitLab Operator could reconcile the changes to the instance. The version should match
   the chart version specified after GitLab Operator upgrade.

#### Troubleshooting

If you notice any errors, first see our
[troubleshooting documentation](troubleshooting.md).
If the answer is not provided there, check for an existing issue or open a new issue in our
[issue tracker](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues).

### Upgrade GitLab by using GitLab Operator

> [!warning]
> By default, the GitLab Operator upgrades GitLab using the [zero-downtime](https://docs.gitlab.com/update/zero_downtime/) approach.
> As a result, GitLab and the underlying chart version should be updated one minor release at a time.
>
> Operator releases before 2.6.0 and 2.5.1 did not enforce a valid upgrade path.
>
> To skip minor versions during an upgrade, you must [upgrade with downtime](#upgrade-with-downtime).

1. Update the `spec.chart.version` field in the GitLab custom resource to a new version. For example:

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

1. Apply the modified GitLab custom resource to the cluster:

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   You should see the following message:

   ```shell
   gitlab.apps.gitlab.com/gitlab created
   ```

   You can watch the progress in the controller logs. For example:

   ```shell
   $ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
   2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
   2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
   ...
   ```

   You see log entries following the upgrade steps outlined above. You can also view the GitLab custom resource status in
   the cluster:

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

### Upgrade with downtime

By default, the GitLab Operator enforces a [zero-downtime upgrade](https://docs.gitlab.com/update/zero_downtime/) path,
which requires updating one minor version at a time. If you prefer to skip minor versions (for example, upgrading from
GitLab 18.0 to 18.2), you can disable zero-downtime upgrades by adding the
`gitlab.io/disable-zero-downtime-upgrade` annotation to the GitLab custom resource.

> [!warning]
> During an upgrade with downtime, the GitLab instance is unavailable while database migrations run.
> Before proceeding, ensure you have planned a maintenance window and have communicated the expected downtime to your users.
>
> You must still follow the required [upgrade stops](https://docs.gitlab.com/update/upgrade_paths/) when skipping
> minor versions. Plan your upgrade path accordingly to ensure you stop at each required version before proceeding
> to your target version.

To perform an upgrade with downtime:

1. Consider [turning on maintenance mode](https://docs.gitlab.com/administration/maintenance_mode/).
1. Perform a [backup](https://docs.gitlab.com/charts/backup-restore/).
1. Upgrade GitLab Operator to a version that supports the target GitLab version.
1. Add the `gitlab.io/disable-zero-downtime-upgrade` annotation and update `spec.chart.version` in the
   GitLab custom resource:

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   + annotations:
   +   gitlab.io/disable-zero-downtime-upgrade: "true"
   spec:
     chart:
   -   version: "9.0.0"
   +   version: "9.2.0"
       values:
         ...
   ```

1. Apply the modified GitLab custom resource:

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   The operator automatically:

   1. Scales down Webservice and Sidekiq deployments to zero replicas.
   1. Waits for all pods to terminate.
   1. Runs database migrations.
   1. Reconciles Webservice and Sidekiq with the new chart version.
   1. Restores Webservice and Sidekiq replica counts from the chart values.

1. Monitor the upgrade progress:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

1. Wait for the upgrade to complete:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS      VERSION
   gitlab   Running     9.3.0
   ```

1. After the upgrade completes, remove the annotation to restore the default zero-downtime upgrade behavior
   for future upgrades:

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   - annotations:
   -   gitlab.io/disable-zero-downtime-upgrade: "true"
   spec:
     chart:
       version: "9.3.0"
       values:
         ...
   ```

1. If enabled, [turn off maintenance mode](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode).

## How GitLab Operator upgrades GitLab

At the beginning of the controller reconcile loop, GitLab Operator checks if the current version matches the required
version.

- If these versions match, then the regular reconcile loop executes, ensuring objects exist that satisfy the
  configuration provided in the custom resource (CR) spec.
- If these versions do not match, the regular reconcile loop still executes, but an additional branch of logic executes
  to handle the upgrade.

In the upgrade:

1. The controller reconciles all deployments. The Webservice and Sidekiq deployments are reconciled but are paused.
   The old pods stay up until the new deployments are resumed.
1. Pre-migrations run, which runs the Migrations job, but skips post-deployment migrations.
1. The controller resumes the Webservice and Sidekiq deployments.
1. The controller waits for the new Webservice and Sidekiq pods to be running.
1. Post-migrations run, which runs the Migrations job without skipping post-deployment migrations.
1. The controller performs a rolling update on the Webservice and Sidekiq deployments.
1. The controller waits for the restarted Webservice and Sidekiq pods to be running.

In future reconcile loops, this branch of logic is skipped because the desired version
(from `spec.chart.version`) matches the current version (from `status.version`).

## Related topics

- [Upgrade Helm chart installations](https://docs.gitlab.com/charts/installation/upgrade/)
- [GitLab Helm chart versions](https://docs.gitlab.com/charts/installation/version_mappings/)
- [Plan your upgrade path](https://docs.gitlab.com/update/upgrade_paths/)
- [GitLab upgrade notes](https://docs.gitlab.com/update/versions/)
- [Changes between GitLab versions](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
