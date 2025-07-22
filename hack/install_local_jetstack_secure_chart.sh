#!/usr/bin/env bash

set -eu -o pipefail

# This script is provided to quickly install the Jetstack Secure Helm chart from the local checkout
# into a Kind cluster, for testing changes to the legacy chart with Jetstack Secure.
#
# This script should be invoked from the root of the repository, e.g.:
# ./hack/install_local_jetstack_secure_chart.sh

TLSPK_ORG="${TLSPK_ORG:-jetstack}"
TLSPK_CLUSTER_NAME="jss_test_$(date +"%Y%m%d_%H%M")"

helm install cert-manager oci://quay.io/jetstack/charts/cert-manager:v1.18.2 \
	--set crds.enabled=true \
	--namespace cert-manager \
	--create-namespace \
	--set 'extraArgs={--dns01-recursive-nameservers-only,--dns01-recursive-nameservers=https://1.1.1.1/dns-query}'

kubectl create namespace jetstack-secure || :

# Get credentials from: https://platform.jetstack.io/org/jetstack/manage/service_accounts
# Save them as JSON a file named credentials.json
kubectl create secret generic agent-credentials --namespace jetstack-secure --from-file=credentials.json || :

helm upgrade --install --create-namespace -n jetstack-secure jetstack-agent \
    ./deploy/charts/jetstack-agent	\
	--set config.organisation="${TLSPK_ORG}" \
	--set config.cluster="${TLSPK_CLUSTER_NAME}"
