#!/usr/bin/env bash
#
# Build and deploy the discovery-agent Helm chart for NGTS.
# Wait for the agent to log a message indicating successful data upload.
#
# Prerequisites:
# * kubectl: https://kubernetes.io/docs/tasks/tools/#kubectl
# * kind: https://kind.sigs.k8s.io/docs/user/quick-start/
# * helm: https://helm.sh/docs/intro/install/
# * jq: https://jqlang.github.io/jq/download/
# * make: https://www.gnu.org/software/make/
#
# You can run `make ngts-test-e2e` which will automatically download all
# prerequisites and then run this script.

set -o nounset
set -o errexit
set -o pipefail

# NGTS API configuration
: ${NGTS_CLIENT_ID?}
: ${NGTS_PRIVATE_KEY?}
: ${NGTS_TSG_ID?}

# The base URL of the OCI registry used for Docker images and Helm charts
# E.g. ttl.sh/7e6ca67c-96dc-4dea-9437-80b0f3a69fb1
: ${OCI_BASE?}

# The Kubernetes namespace to install into
: ${NAMESPACE:=ngts}

# Set to true to use an existing cluster, otherwise a new kind cluster will be created.
# Note: the cluster will not be deleted after the test completes.
: ${USE_EXISTING_CLUSTER:=false}

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
root_dir=$(cd "${script_dir}/../.." && pwd)
export TERM=dumb

tmp_dir="$(mktemp -d /tmp/jetstack-secure.XXXXX)"
trap 'rm -rf "${tmp_dir}"' EXIT

pushd "${tmp_dir}"
> release.env
make -C "$root_dir" ngts-release \
     GITHUB_OUTPUT="${tmp_dir}/release.env" \
     OCI_SIGN_ON_PUSH=false \
     oci_platforms="" \
     NGTS_OCI_BASE="${OCI_BASE}"
cat release.env
source release.env

if [[ "$USE_EXISTING_CLUSTER" != true ]]; then
  kind create cluster || true
fi

kubectl create ns "$NAMESPACE" || true

kubectl delete secret discovery-agent-credentials --namespace "$NAMESPACE" --ignore-not-found
kubectl create secret generic discovery-agent-credentials \
        --namespace "$NAMESPACE" \
        --from-literal=clientID=$NGTS_CLIENT_ID \
        --from-literal=privatekey.pem="$NGTS_PRIVATE_KEY"

# Create a sample secret in the cluster
kubectl create secret generic e2e-sample-secret-$(date '+%s') \
        --namespace default \
        --from-literal=username=${RANDOM}

# Create values.yaml file for the helm chart
cat > "${tmp_dir}/values.yaml" <<EOF
extraArgs:
  - "--log-level=6"

pprof:
  enabled: true

fullnameOverride: discovery-agent

imageRegistry: ${OCI_BASE}
imageNamespace: ""

image:
  digest: ${NGTS_IMAGE_DIGEST}

config:
  clusterName: "e2e-test-cluster-ngts"
  clusterDescription: "A temporary cluster for E2E testing NGTS"
  period: 10s
  tsgID: "${NGTS_TSG_ID}"
  serverURL: "https://${NGTS_TSG_ID}.ngts.dev.venafi.io"

podLabels:
  "discovery-agent.ngts/test-id": "${RANDOM}"
EOF

# Detect running locally on macOS, and if so inject a custom CA bundle to be used
if [[ "$OSTYPE" == "darwin"* ]]; then
  echo "Detected running on macOS - adding system trust bundle to cluster + updating values.yaml to mount in agent pod"

  CA_BUNDLE_FILE=${tmp_dir}/system_certs.pem

  (security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain && \
   security find-certificate -a -p /Library/Keychains/System.keychain) >  $CA_BUNDLE_FILE

  kubectl create configmap custom-ca --namespace="$NAMESPACE" --from-file=ca_certs.crt="$CA_BUNDLE_FILE"

  # Need to update values.yaml to add the custom CA bundle
  custom_ca_yaml="${script_dir}/custom_ca.yaml"
  yq eval-all '. as $item ireduce ({}; . * $item)' "${tmp_dir}/values.yaml" "$custom_ca_yaml" > "${tmp_dir}/values.merged.yaml"
  mv "${tmp_dir}/values.merged.yaml" "${tmp_dir}/values.yaml"
fi

# We use a non-existent tag and omit the `--version` flag, to work around a Helm
# v4 bug. See: https://github.com/helm/helm/issues/31600
helm upgrade agent "oci://${NGTS_CHART}:NON_EXISTENT_TAG@${NGTS_CHART_DIGEST}" \
     --install \
     --wait \
     --create-namespace \
     --namespace "$NAMESPACE" \
     --values "${tmp_dir}/values.yaml"

kubectl rollout status deployments/discovery-agent --namespace "${NAMESPACE}"

# Wait for log message indicating success.
# Parse logs as JSON using jq to ensure logs are all JSON formatted.
timeout 120 jq -n \
        'inputs | if .msg | test("Data sent successfully") then . | halt_error(0) else . end' \
        <(kubectl logs deployments/discovery-agent --namespace "${NAMESPACE}" --follow)

# Query the Prometheus metrics endpoint to ensure it's working.
kubectl get pod \
        --namespace ngts \
        --selector app.kubernetes.io/name=discovery-agent \
        --output jsonpath={.items[*].metadata.name} \
    | xargs -I{} kubectl get --raw /api/v1/namespaces/ngts/pods/{}:8081/proxy/metrics \
    | grep '^process_'

# Query the pprof endpoint to ensure it's working.
kubectl get pod \
        --namespace ngts \
        --selector app.kubernetes.io/name=discovery-agent \
        --output jsonpath={.items[*].metadata.name} \
    | xargs -I{} kubectl get --raw /api/v1/namespaces/ngts/pods/{}:8081/proxy/debug/pprof/cmdline \
    | xargs -0

# TODO: should call to SCM and verify that certs are actually uploaded
