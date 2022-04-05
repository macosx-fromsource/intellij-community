# Security context constraints

## Overview

Pods in OpenShift receive permissions based on their security context
constraints. Security context constraints, often abbreviated _**SCC**_,
simplify the role based access control mechanism for use in large scale
deployments. [Administrators may consult the upstream documentation to gain more insight into how security context constraints work and their place in OpenShift](https://docs.openshift.com/container-platform/4.8/authentication/managing-security-context-constraints.html)

Administrators may also consult the following resources:

1. [Managing Security Context Constraints in OpenShift](https://www.openshift.com/blog/managing-sccs-in-openshift)
1. [A Guide to OpenShift and UIDs](https://www.openshift.com/blog/a-guide-to-openshift-and-uids)

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

The `gitlab-app` ServiceAccount has no granted privileges. It exists solely
to bind the _**anyuid**_ security context constraint to GitLab application
pods.

The security context constraints will be tightened in future releases as the
full _read/write_ behaviors of the GitLab application are validated within
the OpenShift security model.

Note:
Administrators coming to Cloud Native GitLab from Omnibus should note that
Omnibus tasks performed with `sudo` are handled by OpenShift and the
underlying Kubernetes engine. Pods are individual services which, in GitLab
Omnibus, drop privilege to run as an application-specific user. The
**Operator** will [terminate any pod that is not operating with the expected UID](#security-context-constraints-within-the-gitlab-deployment).

### Third party resource definitions

### Ingress controller

GitLab recommends and tests deployments using the
`nginx-ingress-controller` when deploying Cloud Native GitLab. It uses its
own [`nginx-ingress-scc` security context constraint](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/config/openshift/nginx_scc.yaml).

If selecting an alternative Ingress controller, please consult the relevant
documentation to learn more about its security context constraints.

### SSL encryption

**Operator** deploys the [**cert-manager-operator** from JetStack](https://cert-manager.io/docs/installation/openshift/)
to manage SSL certificates across the GitLab application. The
**cert-manager-operator** sets no secure context constraints directly, thus
OpenShift will apply the _**restricted**_ security context constraint by
default.
