trigger_@@CHART@@:
  trigger:
    include: .gitlab-ci.yml
    # strategy: depend
  variables:
    GITLAB_CHART_VERSION: @@CHART@@
    TESTS_NAMESPACE: "${CI_COMMIT_SHORT_SHA}-@@SUFFIX@@"
    HOSTSUFFIX: "${CI_COMMIT_SHORT_SHA}-@@SUFFIX@@"
    ENVIRONMENT_SUFFIX: @@SUFFIX@@
    PIPELINE_TYPE: chart_version
  when: manual
  rules:
    - if: $CI_MERGE_REQUEST_IID
    - if: $CI_COMMIT_BRANCH
    - if: $GITLAB_CHART_VERSION == null
 
