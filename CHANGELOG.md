## 0.26.0 (2023-11-16)

### changed (2 changes)

- [Use Service and PodMonitors from the Helm template](gitlab-org/cloud-native/gitlab-operator@d05b4f16881344e009ceef0623ff4136067f8f0c) ([merge request](gitlab-org/cloud-native/gitlab-operator!661))
- [Migrations for Operator SDK 1.32.0](gitlab-org/cloud-native/gitlab-operator@6c15e3d27d2510f71fae6adabd9c63bffc46cb55) ([merge request](gitlab-org/cloud-native/gitlab-operator!702))

## 0.25.2 (2023-11-14)

No changes.

## 0.25.1 (2023-10-31)

No changes.

## 0.25.0 (2023-10-22)

### added (2 changes)

- [Add Option to Configure Container securityContext](gitlab-org/cloud-native/gitlab-operator@434a77c30f1aa86b7fd7286685d40bc1c8a046d5) by @omland-94 ([merge request](gitlab-org/cloud-native/gitlab-operator!697))
- [Add support for GitLab Zoekt](gitlab-org/cloud-native/gitlab-operator@1dbc8eee8e3754496eb37069e0c2bef0c4c66c96) ([merge request](gitlab-org/cloud-native/gitlab-operator!690))

### changed (1 change)

- [Replace Prometheus object definitions with objects from charts](gitlab-org/cloud-native/gitlab-operator@f21b17dc5b6905e1fac4cfb7e547e5a5e84b9b36) ([merge request](gitlab-org/cloud-native/gitlab-operator!686))

### fixed (3 changes)

- [Populate ACME Ingresse to managed objects](gitlab-org/cloud-native/gitlab-operator@27b69485a7aa4a5392a851774f83110798c25c32) ([merge request](gitlab-org/cloud-native/gitlab-operator!699))
- [Fix modified chart catalog after a chart was rendered](gitlab-org/cloud-native/gitlab-operator@ee2cb71efdfd6ef6c751e6e70cc23220dc32954c) ([merge request](gitlab-org/cloud-native/gitlab-operator!694))
- [Support multiple Services from GitLab Pages](gitlab-org/cloud-native/gitlab-operator@1b6627bfb742f6cc4e81c517233814265a7f4a69) ([merge request](gitlab-org/cloud-native/gitlab-operator!698))

### other (1 change)

- [Replace apimachinery.Scheme with kubectl.Scheme](gitlab-org/cloud-native/gitlab-operator@99286ed2f77244db2a3e44d1d95c06d01cc8ad73) ([merge request](gitlab-org/cloud-native/gitlab-operator!692))

## 0.24.1 (2023-09-28)

No changes.

## 0.24.0 (2023-09-22)

### fixed (1 change)

- [Fix monitoring resources create / delete loop](gitlab-org/cloud-native/gitlab-operator@53f73f98c71f7ef39f4398f08e48a9a2a8bf02e4) ([merge request](gitlab-org/cloud-native/gitlab-operator!681))

### added (1 change)

- [Add Kubernetes 1.26 CI test jobs](gitlab-org/cloud-native/gitlab-operator@8c75c28beee70616affaf255408a370586f857dd) ([merge request](gitlab-org/cloud-native/gitlab-operator!678))

## 0.23.4 (2023-09-18)

No changes.

## 0.23.3 (2023-09-12)

### fixed (1 change)

- [Fix monitoring resources create / delete loop](gitlab-org/cloud-native/gitlab-operator@2de341ae14300b0393366984efaa2d006ef54a88) ([merge request](gitlab-org/cloud-native/gitlab-operator!684))

## 0.23.2 (2023-09-05)

No changes.

## 0.23.1 (2023-08-31)

No changes.

## 0.23.0 (2023-08-22)

### other (2 changes)

- [Update client-go to 0.27.0](gitlab-org/cloud-native/gitlab-operator@8c5237230fb003c32f02d43ac0f0cc13a51fafed) ([merge request](gitlab-org/cloud-native/gitlab-operator!664))
- [Revert Set global.hosts.externalIP on NGINX Service](gitlab-org/cloud-native/gitlab-operator@a6c1db6dde79352029a683bbe55c81959fc2ba9c) ([merge request](gitlab-org/cloud-native/gitlab-operator!652))

### fixed (2 changes)

- [Delete unused resources](gitlab-org/cloud-native/gitlab-operator@14877813ba2b90ea43fa4cc074baf3e14a5433c9) ([merge request](gitlab-org/cloud-native/gitlab-operator!649))
- [Remove override for GitLab Shell Service type](gitlab-org/cloud-native/gitlab-operator@b94d581089873cad6037756eddfe387018fbd622) ([merge request](gitlab-org/cloud-native/gitlab-operator!653))

## 0.22.4 (2023-08-11)

No changes.

## 0.22.3 (2023-08-03)

No changes.

## 0.22.2 (2023-08-01)

No changes.

## 0.22.1 (2023-07-25)

No changes.

## 0.22.0 (2023-07-22)

### fixed (2 changes)

- [Use createOrPatch for Issuer, not createOrUpdate](gitlab-org/cloud-native/gitlab-operator@8163e95ebe4a2aaf672a0e3ece3ef79c5a66197c) ([merge request](gitlab-org/cloud-native/gitlab-operator!650))
- [Discover cluster capabilities and use them in chart](gitlab-org/cloud-native/gitlab-operator@7daf0b8e818cf387d6e55a2e5d904dac8188f9ee) ([merge request](gitlab-org/cloud-native/gitlab-operator!644))

### changed (1 change)

- [Update rbac-proxy to 0.14.1](gitlab-org/cloud-native/gitlab-operator@a135c110fee5b24c29e123710edc41b86f9d98bc) ([merge request](gitlab-org/cloud-native/gitlab-operator!641))

### added (1 change)

- [Add OwnerReference filtering to DiscoverManagedObjects](gitlab-org/cloud-native/gitlab-operator@217882ef6debba68c3b162bc675fc7319378b00b) ([merge request](gitlab-org/cloud-native/gitlab-operator!627))

## 0.21.2 (2023-07-05)

No changes.

## 0.21.1 (2023-06-29)

No changes.

## 0.21.0 (2023-06-22)

### added (2 changes)

- [Add auto-discovery feature to DiscoverManagedObjects](gitlab-org/cloud-native/gitlab-operator@04f1667724606ca5ef479848c4c938b40a75752a) ([merge request](gitlab-org/cloud-native/gitlab-operator!626))
- [Add utility for discovering managed objects](gitlab-org/cloud-native/gitlab-operator@40055c16d8382cf29a9e438ec48e6f622204732b) ([merge request](gitlab-org/cloud-native/gitlab-operator!546))

### fixed (3 changes)

- [Fix review environment DNS labels](gitlab-org/cloud-native/gitlab-operator@31889132e9d91d9f84ea96402219a58d4b293e69) ([merge request](gitlab-org/cloud-native/gitlab-operator!623))
- [Fix PostgreSQL and Redis resource lookup after chart upgrade](gitlab-org/cloud-native/gitlab-operator@708523dcf2ceb055008b9ad096394e1f204d38d2) ([merge request](gitlab-org/cloud-native/gitlab-operator!619))
- [Fix long running reconciliation loop on upgrade](gitlab-org/cloud-native/gitlab-operator@67d1f10f6a38d2450e6e1843a21c88819147ea2c) ([merge request](gitlab-org/cloud-native/gitlab-operator!617))

## 0.20.6 (2023-06-16)

No changes.

## 0.20.5 (2023-06-08)

No changes.

## 0.20.4 (2023-06-07)

No changes.

## 0.20.3 (2023-06-05)

- [Fix PostgreSQL and Redis resource lookup after chart upgrade](gitlab-org/cloud-native/gitlab-operator/@f65e60c6d985e39945224b49bf1fc39b24f7f656) ([merge request](gitlab-org/cloud-native/gitlab-operator!619))

## 0.20.2 (2023-06-05)

No changes.

## 0.20.1 (2023-05-23)

No changes.

## 0.20.0 (2023-05-22)

### other (1 change)

- [Documenting switching back to single connection](gitlab-org/cloud-native/gitlab-operator@77a8854c1f6c7cb58da18c08870fd897c8d2e811) ([merge request](gitlab-org/cloud-native/gitlab-operator!616))

## 0.19.4 (2023-05-17)

No changes.

## 0.19.3 (2023-05-10)

No changes.

## 0.19.2 (2023-05-05)

No changes.

## 0.19.1 (2023-05-02)

No changes.

## 0.19.0 (2023-04-22)

### added (1 change)

- [Add support for nameOverride of redis resources](gitlab-org/cloud-native/gitlab-operator@1dcd2610e1e0e83b04d56497ea837244234898b3) ([merge request](gitlab-org/cloud-native/gitlab-operator!565))

### fixed (1 change)

- [Fix linter issue for doc/openshift_ingress.md](gitlab-org/cloud-native/gitlab-operator@8fc3e4797ead02f58ec01a218b94dda55c3637e0) ([merge request](gitlab-org/cloud-native/gitlab-operator!610))

## 0.18.3 (2023-04-14)

No changes.

## 0.18.2 (2023-04-05)

No changes.

## 0.18.1 (2023-03-30)

No changes.

## 0.18.0 (2023-03-22)

### other (1 change)

- [Distinguish operator default and operator override values](gitlab-org/cloud-native/gitlab-operator@02b42fab33163a2f2434e8a4da0c7a6e26c539dd) ([merge request](gitlab-org/cloud-native/gitlab-operator!586))

## 0.17.3 (2023-03-09)

No changes.

## 0.17.2 (2023-03-02)

No changes.

## 0.17.1 (2023-02-24)

No changes.

## 0.17.0 (2023-02-22)

### other (1 change)

- [Use non-floating golangci-lint image](gitlab-org/cloud-native/gitlab-operator@0f8e5094e5794882e522c10f9d0025d12e115595) ([merge request](gitlab-org/cloud-native/gitlab-operator!584))

### added (1 change)

- [Add support for nameOverride of PostgreSQL resources](gitlab-org/cloud-native/gitlab-operator@993bc94c977c419c827daf4b65dfa65f1bb23fff) ([merge request](gitlab-org/cloud-native/gitlab-operator!570))

### fixed (2 changes)

- [Fail early if Chart catalog cannot be populated](gitlab-org/cloud-native/gitlab-operator@089e4204bf848f24cb918f8b8ddaf549fa723769) ([merge request](gitlab-org/cloud-native/gitlab-operator!572))
- [Truncate secret annotation key](gitlab-org/cloud-native/gitlab-operator@792b8bcf6276fe6d398125268b367b1250d1a8ff) ([merge request](gitlab-org/cloud-native/gitlab-operator!569))

## 0.16.3 (2023-02-15)

No changes.

## 0.16.2 (2023-02-14)

No changes.

## 0.16.1 (2023-01-31)

No changes.

## 0.16.0 (2023-01-22)

### added (1 change)

- [Add OLM bundle testing script with instructions](gitlab-org/cloud-native/gitlab-operator@dd1c21081d8d1152e1cd8c5ca433f093cda4528e) ([merge request](gitlab-org/cloud-native/gitlab-operator!352))

### fixed (2 changes)

- [Support disabling webhook self-signed cert](gitlab-org/cloud-native/gitlab-operator@a14b87b3b9bc31ce860c1c5a7eed63cfbb0613c9) ([merge request](gitlab-org/cloud-native/gitlab-operator!562))
- [Create cert manager resources only when needed ](gitlab-org/cloud-native/gitlab-operator@0f2df982efda0c51e3680c7aaa6a9d07ee270477) by @javion1 ([merge request](gitlab-org/cloud-native/gitlab-operator!561))

## 0.15.5 (2023-01-17)

No changes.

## 0.15.4 (2023-01-17)

No changes.

## 0.15.3 (2023-01-11)

No changes.

## 0.15.2 (2023-01-09)

No changes.

## 0.15.1 (2023-01-05)

No changes.

## 0.15.0 (2022-12-22)

No changes.

## 0.14.2 (2022-12-06)

No changes.

## 0.14.1 (2022-11-30)

No changes.

## 0.14.0 (2022-11-22)

### removed (1 change)

- [Change OpenShift minimum version to 4.8](gitlab-org/cloud-native/gitlab-operator@75a81e8cde8e57ece8a2fd24b42fc5bb6c736e71) ([merge request](gitlab-org/cloud-native/gitlab-operator!545))

### fixed (1 change)

- [Add OCO setup script](gitlab-org/cloud-native/gitlab-operator@14105d6d280ff20bf17183009819ba35c26cae0b) ([merge request](gitlab-org/cloud-native/gitlab-operator!533))

## 0.13.4 (2022-11-14)

No changes.

## 0.13.3 (2022-11-08)

No changes.

## 0.13.2 (2022-11-02)

No changes.

## 0.13.1 (2022-10-24)

No changes.

## 0.13.0 (2022-10-22)

### fixed (1 change)

- [Ensure "Running" phase only set if Condition true](gitlab-org/cloud-native/gitlab-operator@b6f8a80f22b8515fde666ee423e3e01d0994c4bd) ([merge request](gitlab-org/cloud-native/gitlab-operator!539))

### added (4 changes)

- [Add documentation on certified images](gitlab-org/cloud-native/gitlab-operator@fb664e38d788b05e423296a32268233d64509408) ([merge request](gitlab-org/cloud-native/gitlab-operator!537))
- [Support reconciling the spamcheck chart](gitlab-org/cloud-native/gitlab-operator@6e6da19e052a549da62d145d7aa4333252add7e6) ([merge request](gitlab-org/cloud-native/gitlab-operator!536))
- [Support batch/v1beta1 and batch/v1 for CronJob](gitlab-org/cloud-native/gitlab-operator@4a52125d1423c3a13dfbc3b5dfb792234f9445f3) by @Omar007 ([merge request](gitlab-org/cloud-native/gitlab-operator!532))
- [Add new features and components to new GitLab resource adapter](gitlab-org/cloud-native/gitlab-operator@1ae8cda74c28d572b720d4445c514afb6a0b4053) ([merge request](gitlab-org/cloud-native/gitlab-operator!527))

### removed (1 change)

- [Remove the unused custom resource adapter](gitlab-org/cloud-native/gitlab-operator@535d0641b23dfbeee4b221f961f4cb07a6fdc17a) ([merge request](gitlab-org/cloud-native/gitlab-operator!529))

### changed (1 change)

- [Replace the old adapter with the new one](gitlab-org/cloud-native/gitlab-operator@9b9eaf01068087317527518e5722d1e95a67e24f) ([merge request](gitlab-org/cloud-native/gitlab-operator!528))

## 0.12.3 (2022-10-19)

No changes.

## 0.12.2 (2022-10-04)

No changes.

## 0.12.1 (2022-09-29)

No changes.

## 0.12.0 (2022-09-22)

### fixed (1 change)

- [Add fixes from manual run of 0.10.2 certification](gitlab-org/cloud-native/gitlab-operator@7c0368f587c166bb82870e4a33cda9c7ed0eefb9) ([merge request](gitlab-org/cloud-native/gitlab-operator!511))

### performance (1 change)

- [Add `jobSucceeded` method to check Job status](gitlab-org/cloud-native/gitlab-operator@d6c37cf6ed736777a94ac2519c51dcdbac704e49) ([merge request](gitlab-org/cloud-native/gitlab-operator!503))

### other (1 change)

- [Remove NGINX DefaultBackend from tests](gitlab-org/cloud-native/gitlab-operator@1604313afffe233ae427a2092e704647b7bf8f6d) ([merge request](gitlab-org/cloud-native/gitlab-operator!514))

## 0.11.4 (2022-09-05)

No changes.

## 0.11.3 (2022-08-30)

No changes.

## 0.11.2 (2022-08-23)

No changes.

## 0.11.1 (2022-08-22)

No changes.

## 0.11.0 (2022-08-22)

### security (1 change)

- [Add separate nonroot and anyuid RBAC](gitlab-org/cloud-native/gitlab-operator@01d49a714d62cf8d38220e707edc69f9f71a17ce) ([merge request](gitlab-org/cloud-native/gitlab-operator!447))

### added (3 changes)

- [Add Vale configuration and style references](gitlab-org/cloud-native/gitlab-operator@1546a091cd5ad38166314ffb7cc0cdd22df2ff96) ([merge request](gitlab-org/cloud-native/gitlab-operator!509))
- [Script and document RedHat certification process](gitlab-org/cloud-native/gitlab-operator@cdd3b1ed180434e88054079391ca0d0965ccf0f8) ([merge request](gitlab-org/cloud-native/gitlab-operator!494))
- [Add GKE 1.22 jobs](gitlab-org/cloud-native/gitlab-operator@ecdc70c91cf9f14a1eb3dab68135428c2316de69) ([merge request](gitlab-org/cloud-native/gitlab-operator!497))

### changed (1 change)

- [Use project token for RH certification jobs](gitlab-org/cloud-native/gitlab-operator@3cde0d00e1051a306850b102b3b62bd31af7c34a) ([merge request](gitlab-org/cloud-native/gitlab-operator!505))

### fixed (1 change)

- [Deep copy Chart values for catalog query](gitlab-org/cloud-native/gitlab-operator@9b231838685be534e68d40aab69a30cd1970e5c8) ([merge request](gitlab-org/cloud-native/gitlab-operator!499))

## 0.10.1 (2022-07-28)

No changes.

## 0.10.0 (2022-07-22)

### other (1 change)

- [Add .task/ to gitignore](gitlab-org/cloud-native/gitlab-operator@318e1a386eca5970960e166dcab053b1efab9b26) ([merge request](gitlab-org/cloud-native/gitlab-operator!481))

## 0.9.3 (2022-07-19)

No changes.

## 0.9.2 (2022-07-05)

No changes.

## 0.9.1 (2022-06-30)

No changes.

## 0.9.0 (2022-06-22)

No changes.

## 0.8.2 (2022-06-16)

No changes.

## 0.8.1 (2022-06-01)

No changes.

## 0.8.0 (2022-05-22)

No changes.

## 0.7.2 (2022-05-05)

No changes.

## 0.7.1 (2022-05-02)

No changes.

## 0.7.0 (2022-04-22)

No changes.
