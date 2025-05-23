---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Upgrading the Operator
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

Below are instructions to upgrade the GitLab Operator.

Prior to upgrading, it is strongly recommended to [perform a backup](https://docs.gitlab.com/charts/backup-restore/).

## Step 1: Upgrade to the latest available chart version

Before upgrading the Operator, ensure that the current instance of GitLab is upgraded to the latest available chart version by following the
[GitLab upgrade guide](gitlab_upgrades.md). These versions are outlined on the
[releases page](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases) under the `Version mapping` headings.

For example, if the current Operator version is [release 0.4.0](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases/0.4.0),
perform the available chart upgrades _in order_: `5.5.3` -> `5.6.3` -> `5.7.0`.

## Step 2: Identify the desired Operator version

To determine the next GitLab version to upgrade to, refer to the [GitLab upgrade paths](https://docs.gitlab.com/update/#upgrade-paths), and then identify
the GitLab Operator version that supports the desired GitLab version.

See our [releases page](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)
for the full list of available versions of the GitLab Operator.

For example, if the current GitLab Operator version is [`0.4.0`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases/0.4.0), and
the GitLab version that you want to upgrade to is `14.7.1` (chart version `5.7.1`),
you could upgrade to [release 0.4.1](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases/0.4.1).

## Step 3: Install the desired version

The next step is to use `kubectl` to apply the manifest for the desired version of the Operator.

```shell
VERSION=X.Y.Z
kubectl apply -f \
  https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
```

This command applies any changes to the related manifests, including the new Deployment image to use.

## Step 4: Confirm that the new version of the Operator becomes the leader

The Operator Deployment should create a new ReplicaSet with this change, which will spawn a new Operator pod. Meanwhile, the previous
Operator pod will shut down, giving up its leader status. When this happens, the new Operator pod will become the leader.

## Step 5: Update the chart version in the GitLab custom resource (CR)

In most cases, the available chart versions will not be identical between versions of the Operator. When the newer version of the
Operator starts, it will try to reconcile the existing GitLab custom resource (CR). You will likely see an error such as:

```plaintext
Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
```

To address this, identify a valid version from that release's available chart versions.

For example: when upgrading from Operator `0.4.0` to `0.4.1`, update the GitLab CR to an available chart version closest to `5.7.0`, which in this case is `5.7.1`.

## Step 6: Confirm that the Operator reconciles GitLab as expected

Watch the logs from the new Operator pod. You should see that it performs the upgrade to the chart version you defined.

To confirm that the upgrade was successful, get the status of the GitLab CR:

```plaintext
$ kubectl get gitlabs -n gitlab-system
NAME     STATUS    VERSION
gitlab   Running   5.7.1
```

The status `Running` means that the Operator was able to reconcile the changes to the instance. The version should match the chart version
specified after the Operator upgrade.

If you notice any errors, first reference our
[troubleshooting documentation](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/troubleshooting.md).
If the answer is not provided there, please check for an existing issue or open a new issue in our
[issue tracker](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues).

## Related reading

Below are resources related to GitLab upgrades.

- [(Operator) GitLab Upgrades](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/gitlab_upgrades.md)
- [(Charts) Upgrade guide](https://docs.gitlab.com/charts/installation/upgrade/)
- [(Charts) Version mappings](https://docs.gitlab.com/charts/installation/version_mappings/)
- [GitLab upgrade paths](https://docs.gitlab.com/update/#upgrade-paths)
- [GitLab version-specific changes](https://docs.gitlab.com/update/package/#version-specific-changes)
- [Changes between GitLab versions](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
