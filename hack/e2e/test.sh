#!/usr/bin/env bash

# Prerequisites
# * https://github.com/ko-build/ko/releases/tag/v0.16.0

set -o nounset
set -o errexit
set -o pipefail
set -o xtrace

: ${VEN_API_KEY?}
: ${VEN_OWNING_TEAM?}

script_dir=$(cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd)
root_dir=$(cd "${script_dir}/../.." && pwd)

cd "${script_dir}"

export VERSION=0.1.49
export TERM=dumb
OCI_BASE=ttl.sh/63773370-0bcf-4ac0-bd42-5515616089ff
export KO_DOCKER_REPO=$OCI_BASE/images/venafi-agent

pushd "${root_dir}"
ko build  --bare --tags "v${VERSION}"
helm package deploy/charts/venafi-kubernetes-agent --version "${VERSION}"
helm push venafi-kubernetes-agent-${VERSION}.tgz "oci://${OCI_BASE}/charts"
popd

kind create cluster || true

kubectl create ns venafi || true

# Pull secret for Venafi OCI registry
if ! kubectl get secret venafi-image-pull-secret -n venafi; then
    venctl iam service-accounts registry create \
           --no-prompts \
           --owning-team "${VEN_OWNING_TEAM}" \
           --name "venafi-kubernetes-agent-e2e-registry-${RANDOM}" \
           --scopes enterprise-cert-manager,enterprise-venafi-issuer,enterprise-approver-policy \
    | jq '{
            "apiVersion": "v1",
            "kind": "Secret",
            "metadata": {
              "name": "venafi-image-pull-secret"
            },
            "type": "kubernetes.io/dockerconfigjson",
            "stringData": {
              ".dockerconfigjson": {
                "auths": {
                  "\(.oci_registry)": {
                    "username": .username,
                    "password": .password
                  }
                }
              } | tostring
            }
          }' \
    | kubectl create -n venafi -f -
fi

# Cache the Service account credentials for venafi-kubernetes-agent in the cluster
# but this Secret will not be mounted by the agent.
kubectl create ns venafi-kubernetes-agent-e2e || true
if ! kubectl get secret cached-venafi-agent-service-account -n venafi-kubernetes-agent-e2e; then
    venctl iam service-account agent create \
           --no-prompts \
           --owning-team "${VEN_OWNING_TEAM}" \
           --name "venafi-kubernetes-agent-e2e-agent-${RANDOM}" \
    | jq '{
            "apiVersion": "v1",
            "kind": "Secret",
            "metadata": {
              "name": "cached-venafi-agent-service-account"
            },
            "stringData": {
              "privatekey.pem": .private_key,
              "client-id": .client_id
            }
          }' \
    | kubectl create -n venafi-kubernetes-agent-e2e -f -
fi

export VENAFI_KUBERNETES_AGENT_CLIENT_ID="not-used-but-required-by-venctl"
venctl components kubernetes apply \
       --venafi-kubernetes-agent \
       --venafi-kubernetes-agent-version "$VERSION" \
       --venafi-kubernetes-agent-values-files "${script_dir}/values.venafi-kubernetes-agent.yaml" \
       --venafi-kubernetes-agent-custom-image-registry "${OCI_BASE}/images" \
       --venafi-kubernetes-agent-custom-chart-repository "oci://${OCI_BASE}/charts"

privatekey=$(kubectl get secret cached-venafi-agent-service-account \
                     --namespace venafi-kubernetes-agent-e2e \
                     --template="{{index .data \"privatekey.pem\" | base64decode}}")
clientid=$(kubectl get secret cached-venafi-agent-service-account \
                   --namespace venafi-kubernetes-agent-e2e \
                   --template="{{index .data \"client-id\" | base64decode}}")
jwt=$(step crypto jwt sign \
                   --key <(sed 's/ PRIVATE KEY/ EC PRIVATE KEY/g' <<<"$privatekey") \
                   --aud api.venafi.cloud/v1/oauth/token/serviceaccount \
                   --exp "$(date -d '+30 minutes' +'%s')" \
                   --sub "$clientid" \
                   --iss "$clientid" \
                  | tee >(step crypto jwt inspect --insecure >/dev/stderr))
accesstoken=$(curl https://api.venafi.cloud/v1/oauth/token/serviceaccount \
             -sS --fail-with-body \
             --data-urlencode assertion="$jwt" \
             --data-urlencode grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer | tee /dev/stderr | jq '.access_token' -r)
export accesstoken
envsubst < venafi-components.yaml | kubectl apply -n venafi -f -
