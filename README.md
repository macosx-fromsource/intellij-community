[![Go Report Card](https://goreportcard.com/badge/gitlab.com/gitlab-org/cloud-native/gitlab-operator "Go Report Card")](https://goreportcard.com/report/gitlab.com/gitlab-org/cloud-native/gitlab-operator)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Reference](https://pkg.go.dev/badge/gitlab.com/gitlab-org/cloud-native/gitlab-operator.svg)](https://pkg.go.dev/gitlab.com/gitlab-org/cloud-native/gitlab-operator)

# GitLab Operator

**Note:** The GitLab Operator has [known limitations](doc/_index.md#known-issues) and is
only suitable for specific scenarios in production use. For more details see the
[install guide](doc/installation.md).

The GitLab Operator aims to manage the full lifecycle of GitLab instances on your Kubernetes
or OpenShift infrastructure.

The GitLab Operator aims to:

- Ease installation and configuration of GitLab instances.
- Offer seamless upgrades from version to version.
- Ease backup and restore of GitLab and its components.
- Aggregate metrics using Prometheus.
- Setup auto-scaling.

Note that this Operator does not deploy GitLab Runner. To deploy GitLab Runner, see the
[GitLab Runner Operator repository](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator).

## Documentation

Information on installation, usage, and contributing to the GitLab Operator can be found
at the [documentation site](https://docs.gitlab.com/operator/).

## Owned Custom Resource: GitLab

The operator is responsible for owning, watching, and reconciling the GitLab custom resource.

An example GitLab object is shown below:

```yaml
apiVersion: apps.gitlab.com/v1beta1
kind: GitLab
metadata:
  name: gitlab
spec:
  chart:
    version: "X.Y.Z"
    values:
      global:
        hosts:
          domain: example.com
```
