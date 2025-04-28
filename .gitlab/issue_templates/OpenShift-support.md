## References

- [OpenShift release post](<insert link here>)
- [Parent Epic: Support deploying GitLab on new versions of OpenShift](https://gitlab.com/groups/gitlab-org/-/epics/13683)
- [GitLab OpenShift release support policy](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)
- [Operator supported OpenShift releases](https://docs.gitlab.com/operator/installation/?tab=OpenShift#cluster)


## Checklist

Follow the checklist below to address the requirements for each project.

### Administrative

- [ ] Change issue title to: `Support deploying GitLab to OpenShift X.Y`
- [ ] Insert the OpenShift release post URL above

### [GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator)

- [ ] Replace any removed APIs
- [ ] Create new OpenShift cluster on GCP
  - [ ] Create a new cluster via the [openshift-provisioning project][openshift-provisioning]
  - [ ] Save the Kubeconfig file in 1Password in the `Cloud Native` group in a new item named `ocp-ci-XYZ cluster credentials`
  - [ ] Create a new [Operator project CI variable](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/settings/ci_cd)
        with the Kubeconfig file. See existing variables for reference.
- [ ] Add CI jobs to test against the new cluster
- [ ] Update kubectl to `1.N.x` in the [CI base image](https://gitlab.com/gitlab-org/gitlab-build-images/-/blob/master/Dockerfile.gitlab-operator-build-base-golang-1.22)
- [ ] Remove old cluster CI jobs and clusters
  - [ ] Remove CI testing jobs for deprecated OpenShift clusters
  - [ ] Remove out-of-support OpenShift clusters
- [ ] Update supported releases documentation
  - [ ] Add entry for newly-supported OpenShift version
  - [ ] Deprecate OpenShift versions that have reached end-of-life
  - [ ] Remove entries that are no longer supported


/label ~"devops::gitlab delivery" ~"group::Self Managed" ~"section::infrastructure platforms"

/label ~"type::maintenance" ~"maintenance::pipelines" ~"For scheduling" ~"priority::2"

/epic https://gitlab.com/groups/gitlab-org/-/epics/13683

[openshift-provisioning]: https://gitlab.com/gitlab-org/distribution/infrastructure/openshift-provisioning
