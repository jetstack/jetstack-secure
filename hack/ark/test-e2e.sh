#!/usr/bin/env bash
#
# Build and deploy the disco-agent Helm chart.
# Wait for the agent to log a message indicating successful data upload.
#
# Prerequisites:
# * kubectl: https://kubernetes.io/docs/tasks/tools/#kubectl
# * kind: https://kind.sigs.k8s.io/docs/user/quick-start/
# * helm: https://helm.sh/docs/intro/install/
# * jq: https://jqlang.github.io/jq/download/
# * make: https://www.gnu.org/software/make/
#
# You can run `make ark-test-e2e` which will automatically download all
# prerequisites and then run this script.

set -o nounset
set -o errexit
set -o pipefail

# CyberArk API configuration
: ${ARK_USERNAME?}
: ${ARK_SECRET?}
: ${ARK_SUBDOMAIN?}
: ${ARK_DISCOVERY_API?}

# The base URL of the OCI registry used for Docker images and Helm charts
# E.g. ttl.sh/7e6ca67c-96dc-4dea-9437-80b0f3a69fb1
: ${OCI_BASE?}

# The Kubernetes namespace to install into
: ${NAMESPACE:=cyberark}

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
make -C "$root_dir" ark-release \
     GITHUB_OUTPUT="${tmp_dir}/release.env" \
     OCI_SIGN_ON_PUSH=false \
     oci_platforms="" \
     ARK_OCI_BASE="${OCI_BASE}"
cat release.env
source release.env

if [[ "$USE_EXISTING_CLUSTER" != true ]]; then
  kind create cluster || true
fi

kubectl create ns "$NAMESPACE" || true

kubectl delete secret agent-credentials --namespace "$NAMESPACE" --ignore-not-found
kubectl create secret generic agent-credentials \
        --namespace "$NAMESPACE" \
        --from-literal=ARK_USERNAME=$ARK_USERNAME \
        --from-literal=ARK_SECRET=$ARK_SECRET \
        --from-literal=ARK_SUBDOMAIN=$ARK_SUBDOMAIN \
        --from-literal=ARK_DISCOVERY_API=$ARK_DISCOVERY_API

helm upgrade agent "oci://${ARK_CHART}@${ARK_CHART_DIGEST}" \
     --version "${ARK_CHART_TAG}" \
     --install \
     --wait \
     --create-namespace \
     --namespace "$NAMESPACE" \
     --set-json extraArgs='["--log-level=6"]' \
     --set pprof.enabled=true \
     --set fullnameOverride=disco-agent \
     --set "image.digest=${ARK_IMAGE_DIGEST}" \
     --set config.clusterDescription="A temporary cluster for E2E testing. Contact @wallrj-cyberark." \
     --set-json "podLabels={\"disco-agent.cyberark.cloud/test-id\": \"${RANDOM}\"}"

kubectl rollout status deployments/disco-agent --namespace "${NAMESPACE}"

# Wait 60s for log message indicating success.
# Parse logs as JSON using jq to ensure logs are all JSON formatted.
timeout 60 jq -n \
        'inputs | if .msg | test("Data sent successfully") then . | halt_error(0) else . end' \
        <(kubectl logs deployments/disco-agent --namespace "${NAMESPACE}" --follow)

# Query the Prometheus metrics endpoint to ensure it's working.
kubectl get pod \
        --namespace cyberark \
        --selector app.kubernetes.io/name=disco-agent \
        --output jsonpath={.items[*].metadata.name} \
    | xargs -I{} kubectl get --raw /api/v1/namespaces/cyberark/pods/{}:8081/proxy/metrics \
    | grep '^process_'

# Query the pprof endpoint to ensure it's working.
kubectl get pod \
        --namespace cyberark \
        --selector app.kubernetes.io/name=disco-agent \
        --output jsonpath={.items[*].metadata.name} \
    | xargs -I{} kubectl get --raw /api/v1/namespaces/cyberark/pods/{}:8081/proxy/debug/pprof/cmdline \
    | xargs -0

