---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Security context constraints

## Overview

Pods in OpenShift receive permissions based on their security context
constraints. Security context constraints, often abbreviated _**SCC**_,
simplify the role based access control mechanism for use in large scale
deployments. [Administrators may consult the upstream documentation to gain more insight into how security context constraints work and their place in OpenShift](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html)

Administrators may also consult the following resources:

1. [Managing Security Context Constraints in OpenShift](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [A Guide to OpenShift and UIDs](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## Security context constraints within the GitLab deployment

The `gitlab-controller-manager` deployment creates and manages the pod
containing the **Operator** processes. This and any other pod it creates and
manages run with the _**restricted**_ security context constraint.

The **Operator** uses a ServiceAccount with robust permissions allowing it
to manage all resources required by the GitLab application.

The **Operator** manages the component services comprising Cloud Native
GitLab. It actively terminates and replaces pods that do not conform to the
UID specified by the **Operator**. This mechanism enforces the principle of
least privilege.

### GitLab application custom resource definitions

Pods deployed by the Operator to satisfy GitLab custom resources use the
_**anyuid**_ security context constraint. Security context constraints for
third party operators and resources are [covered in the next section](#third-party-resource-definitions).

The `gitlab-app-anyuid` and `gitlab-app-nonroot` ServiceAccounts have no granted privileges. They exists solely
to bind the _**anyuid**_ and _**nonroot**_ security context constraints to GitLab application
pods.

The security context constraints will be tightened in future releases as the
full _read/write_ behaviors of the GitLab application are validated within
the OpenShift security model.

NOTE:
Administrators coming to Cloud Native GitLab from a Linux package installation should note that
Linux package installation tasks performed with `sudo` are handled by OpenShift and the
underlying Kubernetes engine. Pods are individual services which, in a Linux package installation,
drop privilege to run as an application-specific user. The
**Operator** will [terminate any pod that is not operating with the expected UID](#security-context-constraints-within-the-gitlab-deployment).

### Third party resource definitions

### Ingress controller

GitLab recommends and tests deployments using the
`nginx-ingress-controller` when deploying Cloud Native GitLab. It uses its
own [`nginx-ingress-scc` security context constraint](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml).

If selecting an alternative Ingress controller, please consult the relevant
documentation to learn more about its security context constraints.

### SSL encryption

**Operator** deploys the [**cert-manager-operator** from JetStack](https://cert-manager.io/docs/releases/)
to manage SSL certificates across the GitLab application. The
**cert-manager-operator** sets no secure context constraints directly, thus
OpenShift will apply the _**restricted**_ security context constraint by
default.
