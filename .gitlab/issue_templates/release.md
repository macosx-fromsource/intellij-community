## Summary

This issue will serve as a checklist and record of releases for GitLab Operator `X.Y.Z`.

Much of the process is automated by [release-tools](https://gitlab.com/gitlab-org/release-tools).

## To do

1. [ ] Confirm that publish jobs in the
       [tag pipeline in Canonical repo](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/pipelines?ref=X.Y.Z)
       has completed.
1. [ ] Confirm that the [Release](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases) has been created
       with the correct content in the tag message and Release description..
1. [ ] [OLM bundle testing](doc/developer/test_olm.md) has been completed
1. [ ] [Release to OperatorHub.io](doc/developer/operatorhub_publishing.md).
1. [ ] [Release to OpenShift OperatorHub.io](doc/developer/operatorhub_publishing.md)
1. [ ] [Submit OLM bundle for OpenShift certification](doc/developer/redhat_certification.md)

Using [publish.sh](scripts/tools/publish.sh) script is recommended for the release-related steps:

`scripts/tools/publish.sh VERSION [TARGETS]`

This scripts checks the requirements and submits the Operator to all three targets.
You can limit the scope of the release by passing one or more targets. Use:

- `operatorhub` for `OperatorHub.io`
- `redhat-community` for `OpenShift OperatorHub`
- `redhat-marketplace` for `OpenShift Certified Operators`

/assign @clemensbeck @pursultani
