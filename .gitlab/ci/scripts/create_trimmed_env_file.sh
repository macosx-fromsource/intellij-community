#!/bin/bash
# Export trimmed values into .env file for further export/consumption
# by other jobs
DOT_ENV_FILE=${1:-"deploy_namespace.env"}
cat << END_ENV | tee > $1
TESTS_NAMESPACE=${TESTS_NAMESPACE}
HOSTSUFFIX=${HOSTSUFFIX}
OLD_HOSTSUFFIX=${OLD_HOSTSUFFIX}
OLD_TESTS_NAMESPACE=${OLD_TESTS_NAMESPACE}
END_ENV