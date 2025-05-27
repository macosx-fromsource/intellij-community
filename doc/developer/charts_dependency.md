---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Dependency on GitLab Charts
---

The GitLab Operator (also just known as "Operator") depends on the [GitLab Helm Charts](https://gitlab.com/gitlab-org/charts/gitlab) (also just known as "Chart").
as described in
[ADR 0004](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr/0004-integration-of-the-gitlab-chart.md).

## Understanding impact from changes to GitLab Chart

When Operator ingests new versions of Chart,
it also ingests the changes within Chart. Sometimes adjustments
must be made to Operator to support the changes in Chart.

For example, Chart [merge request 3278](https://gitlab.com/gitlab-org/charts/gitlab/-/merge_requests/3278)
updated the version of the NGINX Ingress Controller image tag, along with an
update to the NGINX RBAC objects to support the change. This caused Operator
[issue 1324](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1324),
where the pipeline on `master` was broken. The problem was resolved by Operator
[merge request 655](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/655).

When the Chart merge request was submitted, ideally an accompanying merge request in the Operator
should have been opened and marked as dependent on the Chart merge request. This
approach ensures that changes to Chart are considered in the context of Operator,
which helps the team ensure that the two components work together as seamlessly as possible.

## Evaluating impact from changes to GitLab Chart

The impact to Operator must be considered when submitting a change to Chart. This
is included as an item in the approval checklist of Chart merge request
template as a reminder.

{{< alert type="note" >}}

Providing an automated mechanism of testing this is
being investigated in Chart [issue 4900](https://gitlab.com/gitlab-org/charts/gitlab/-/issues/4900).

{{< /alert >}}

To evaluate the impact of changes to Chart on Operator, consider
whether the change will be automatically ingested by Operator or not. The
only way to accomplish this with certainty is to inspect the Operator codebase
manually, searching for related references to resources that are sourced from
Chart, and see if Operator interacts with the related change in any way
that requires adjustment.

In the meantime, the following examples are considered risky:

- Changes to resource naming or labeling scheme, such as `.metadata.name` and/or `.metadata.labels`
- Changes to resource group and/or version, such as `.apiVersion`
- Adding new resources.
- Changing ServiceAccount names or RBAC policies
- Changing security contexts.
- Upgrading Chart dependencies, such as Redis or PostgreSQL chart versions
- Chart-breaking changes that are introduced in major releases or stop versions

Examples of low risk changes are:

- Changing default values.
- Adding new configuration keys to existing ConfigMaps.
- Adding new environment variables to existing resources.

An example of a change that is automatically ingested is Chart
[merge request 3247](https://gitlab.com/gitlab-org/charts/gitlab/-/merge_requests/3247). It added a new field
inside of resources that Operator does not manipulate directly, only retrieving them from
the rendered Helm template and reconciling them in the cluster.

If you consider a MR risky, please request a review from an Operator
reviewer or maintainer.
