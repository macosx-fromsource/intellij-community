#!/bin/bash -e

# Environment variables
ENVIRONMENT="${ENVIRONMENT:-kubernetes}"
CLUSTER_NAME="${CLUSTER_NAME:-ocp-$USER}"
BASE_DOMAIN="${BASE_DOMAIN:-k8s-ft.win}"
RELEASE_NAME="${RELEASE_NAME:-gitlab-external-dns}"
SECRET_NAME="${SECRET_NAME:-$RELEASE_NAME-secret}"
NAMESPACE="${NAMESPACE:-default}"
ZONE_ID="${ZONE_ID:-k8s-ftw}"
PROVIDER="${PROVIDER:-google}"
VERSION="${VERSION:-6.5.2}"
GOOGLE_APPLICATION_CREDENTIALS="${GOOGLE_APPLICATION_CREDENTIALS:-gcloud.json}"
GOOGLE_CREDENTIALS="${GOOGLE_CREDENTIALS:-$(cat $GOOGLE_APPLICATION_CREDENTIALS)}"
GCP_PROJECT_ID="${GCP_PROJECT_ID:-cloud-native-182609}"

export KUBECONFIG="install-$CLUSTER_NAME/auth/kubeconfig"

# Patch the OpenShift Ingress Controller to only watch OpenShift namespaces.
# This prevents OpenShift Ingress Controller from overriding Ingress' `HOSTS`
# field from an IP address to the cluster base domain/hostname. We need IP addresses
# for external-DNS to correctly add A records to the NGINX Service external IP.
if [ "${ENVIRONMENT}" == "openshift" ]; then
  kubectl -n openshift-ingress-operator \
    patch ingresscontroller default \
    --type merge \
    -p '{"spec":{"namespaceSelector":{"matchLabels":{"openshift.io/cluster-monitoring":"true"}}}}'
fi

# Ensure secret with Google credentials exists
kubectl -n "${NAMESPACE}" get secret "${SECRET_NAME}" \
  || kubectl -n "${NAMESPACE}" create secret generic \
       "${RELEASE_NAME}-secret" \
       --from-literal="credentials.json=${GOOGLE_CREDENTIALS}"

# Ensure bitnami helm repo is added
helm repo add bitnami https://charts.bitnami.com/bitnami

# Ensure external-dns chart release is up to date.
# Note: ensure that `txtOwnerID` is unique between installations
# to ensure that one instance of external-dns does not remove records
# created by another instance of external-dns.
helm upgrade --install "${RELEASE_NAME}" bitnami/external-dns \
  --namespace "${NAMESPACE}" \
  --version "${VERSION}" \
  --set provider="${PROVIDER}" \
  --set google.project=${GCP_PROJECT_ID} \
  --set google.serviceAccountSecret="${SECRET_NAME}" \
  --set domainFilters[0]="${BASE_DOMAIN}" \
  --set zoneIdFilters[0]="${ZONE_ID}" \
  --set txtOwnerId="${PROVIDER}-${ENVIRONMENT}-${CLUSTER_NAME}-${NAMESPACE}" \
  --set rbac.create='true' \
  --set policy='sync'
