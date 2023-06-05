#!/bin/bash
echo "Initial HOSTSUFFIX ${HOSTSUFFIX}"
echo "Initial TESTS_NAMESPACE ${TESTS_NAMESPACE}"
export OLD_HOSTSUFFIX=${HOSTSUFFIX}
export OLD_TESTS_NAMESPACE=${TESTS_NAMESPACE}

# Make sure the K8s namespace and DNS names are valid.
# DNS name components: <service_name>-<TESTS_NAMESPACE>.<cluster subdomain>.k8s-ft.win
#   A valid DNS label has a maximum length of 63 characters.
#   The maximum service name length is 8 (registry).
#   TESTS_NAMESPACE contains: <commit_sha>-<branch_name>
#   For a valid DNS name TEST_NAMESPACES must be trimmed to:
#     63 (max DNS label) - 8 (max service name) - 1 (hyphen) = 54 characters
export TESTS_NAMESPACE=${TESTS_NAMESPACE:0:54}
export HOSTSUFFIX=${HOSTSUFFIX:0:54}
echo "Trimmed HOSTSUFFIX ${HOSTSUFFIX}"
echo "Trimmed TESTS_NAMESPACE ${TESTS_NAMESPACE}"
