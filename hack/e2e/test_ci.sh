#!/usr/bin/env bash
#
# Build and install venafi-kubernetes-agent for VenafiConnection based authentication.
# Wait for it to log a message indicating successful data upload.
#
# This script is designed to be executed by a `make` target that has already
# provisioned a Kubernetes cluster (e.g., via `make kind-cluster`).
# It assumes `kubectl` is pre-configured to point to the correct test cluster.
#
# A VenafiConnection resource is created which uses workload identity federation.
#
# Prerequisites (expected to be available in the execution environment):
# * kubectl, venctl, jq, step, curl, envsubst, docker

set -o nounset
set -o errexit
set -o pipefail
# Commenting out for CI, uncomment for local debugging
#set -o xtrace

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
root_dir=$(cd "${script_dir}/../.." && pwd)
export TERM=dumb

# Your Venafi Cloud API key.
: ${VEN_API_KEY?}
# Separate API Key for getting a pull secret.
: ${VEN_API_KEY_PULL?}
# The Venafi Cloud zone.
: ${VEN_ZONE?}
# The hostname of the Venafi API server (e.g., api.venafi.cloud).
: ${VEN_API_HOST?}
# The region of the Venafi API server (e.g., "us" or "eu").
: ${VEN_VCP_REGION?}
# The base URL of the OCI registry (e.g., ttl.sh/some-random-uuid).
: ${OCI_BASE?}

REMOTE_AGENT_IMAGE="${OCI_BASE}/venafi-kubernetes-agent-e2e"

cd "${script_dir}"

# Build and PUSH agent image and Helm chart to the anonymous registry
echo ">>> Building and pushing agent to '${REMOTE_AGENT_IMAGE}'..."
pushd "${root_dir}"
> release.env
make release \
     OCI_SIGN_ON_PUSH=false \
     oci_platforms=linux/amd64 \
     oci_preflight_image_name=${REMOTE_AGENT_IMAGE} \
     helm_chart_image_name=$OCI_BASE/charts/venafi-kubernetes-agent \
     GITHUB_OUTPUT=release.env
source release.env
popd

AGENT_IMAGE_WITH_TAG="${REMOTE_AGENT_IMAGE}:${RELEASE_HELM_CHART_VERSION}"
echo ">>> Successfully pushed image: ${AGENT_IMAGE_WITH_TAG}"

kubectl create ns venafi || true

# Create pull secret for Venafi's OCI registry if it doesn't exist.
if ! kubectl get secret venafi-image-pull-secret -n venafi; then
  echo ">>> Creating Venafi OCI registry pull secret..."
  venctl iam service-accounts registry create \
    --api-key $VEN_API_KEY_PULL \
    --no-prompts \
    --owning-team "$(curl --fail-with-body -sS "https://${VEN_API_HOST}/v1/teams" -H "tppl-api-key: ${VEN_API_KEY_PULL}" | jq '.teams[0].id' -r)" \
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

echo ">>> Generating temporary Helm values for the custom agent image..."
cat <<EOF > /tmp/agent-image-values.yaml
image:
  repository: ${REMOTE_AGENT_IMAGE}
  tag: ${RELEASE_HELM_CHART_VERSION}
  pullPolicy: IfNotPresent
EOF

echo ">>> Applying Venafi components to the cluster..."
export VENAFI_KUBERNETES_AGENT_CLIENT_ID="not-used-but-required-by-venctl"
venctl components kubernetes apply \
  --region $VEN_VCP_REGION \
  --cert-manager \
  --venafi-enhanced-issuer \
  --approver-policy-enterprise \
  --venafi-kubernetes-agent \
  --venafi-kubernetes-agent-version "${RELEASE_HELM_CHART_VERSION}" \
  --venafi-kubernetes-agent-values-files "${script_dir}/values.venafi-kubernetes-agent.yaml" \
  --venafi-kubernetes-agent-values-files "/tmp/agent-image-values.yaml" \
  --venafi-kubernetes-agent-custom-chart-repository "oci://${OCI_BASE}/charts"

kubectl apply -n venafi -f venafi-components.yaml

# Configure Workload Identity Federation with Venafi Cloud
echo ">>> Configuring Workload Identity Federation..."
subject="system:serviceaccount:venafi:venafi-components"
audience="https://${VEN_API_HOST}"
issuerURL=$(kubectl get --raw /.well-known/openid-configuration | jq -r '.issuer')
openidDiscoveryURL="${issuerURL}/.well-known/openid-configuration"
jwksURI=$(curl --fail-with-body -sSL ${openidDiscoveryURL} | jq -r '.jwks_uri')

# Create the Venafi agent service account if one does not already exist
echo ">>> Ensuring Venafi Cloud service account exists for the agent..."
while true; do
  tenantID=$(curl --fail-with-body -sSL -H "tppl-api-key: $VEN_API_KEY" https://${VEN_API_HOST}/v1/serviceaccounts \
    | jq -r '.[] | select(.issuerURL==$issuerURL and .subject == $subject) | .companyId' \
      --arg issuerURL "${issuerURL}" \
      --arg subject "${subject}")

  if [[ "${tenantID}" != "" ]]; then
    echo "Service account already exists."
    break
  fi

  echo "Service account not found, creating it..."
  jq -n '{
      "name": "venafi-kubernetes-agent-e2e-agent-\($random)",
      "authenticationType": "rsaKeyFederated",
      "scopes": ["kubernetes-discovery-federated", "certificate-issuance"],
      "subject": $subject,
      "audience": $audience,
      "issuerURL": $issuerURL,
      "jwksURI": $jwksURI,
      "owner": $owningTeamID
    }' \
    --arg random "${RANDOM}" \
    --arg subject "${subject}" \
    --arg audience "${audience}" \
    --arg issuerURL "${issuerURL}" \
    --arg jwksURI "${jwksURI}" \
    --arg owningTeamID "$(curl --fail-with-body -sS "https://${VEN_API_HOST}/v1/teams" -H "tppl-api-key: $VEN_API_KEY" | jq '.teams[0].id' -r)" \
    | curl "https://${VEN_API_HOST}/v1/serviceaccounts" \
      -H "tppl-api-key: $VEN_API_KEY" \
      --fail-with-body \
      -sSL --json @-
done

# Create the VenafiConnection resource
echo ">>> Applying VenafiConnection resource..."
kubectl apply -n venafi -f - <<EOF
apiVersion: jetstack.io/v1alpha1
kind: VenafiConnection
metadata:
  name: venafi-components
spec:
  allowReferencesFrom: {}
  vcp:
    url: https://${VEN_API_HOST}
    accessToken:
    - serviceAccountToken:
        name: venafi-components
        audiences:
        - ${audience}
    - vcpOAuth:
        tenantID: ${tenantID}
EOF

# Test certificate issuance
echo ">>> Testing certificate issuance..."
envsubst <application-team-1.yaml | kubectl apply -f -
kubectl -n team-1 wait certificate app-0 --for=condition=Ready --timeout=5m

# Wait for the agent to successfully send data to Venafi Cloud
echo ">>> Waiting for agent log message confirming successful data upload..."
set +o pipefail
kubectl logs deployments/venafi-kubernetes-agent \
        --follow \
        --namespace venafi \
    | timeout 60s jq 'if .msg | test("Data sent successfully") then . | halt_error(0) end'
set -o pipefail

# Create a unique TLS secret and verify its discovery by the agent
echo ">>> Testing discovery of a manually created TLS secret..."
commonname="venafi-kubernetes-agent-e2e.$(uuidgen | tr '[:upper:]' '[:lower:]')"
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout /tmp/tls.key -out /tmp/tls.crt -subj "/CN=$commonname"
kubectl create secret tls "$commonname" --cert=/tmp/tls.crt --key=/tmp/tls.key -o yaml --dry-run=client | kubectl apply -f -

getCertificate() {
    jq -n '{
        "expression": {
            "field": "subjectCN",
            "operator": "MATCH",
            "value": $commonname
        },
        "ordering": {
            "orders": [
                { "direction": "DESC", "field": "certificatInstanceModificationDate" }
            ]
        },
        "paging": { "pageNumber": 0, "pageSize": 10 }
    }' --arg commonname "${commonname}" \
    | curl "https://${VEN_API_HOST}/outagedetection/v1/certificatesearch?excludeSupersededInstances=true&ownershipTree=true" \
         -fsSL \
         -H "tppl-api-key: $VEN_API_KEY" \
         --json @- \
    | jq 'if .count == 0 then . | halt_error(1) end'
}

# Wait up to 5 minutes for the certificate to appear in the Venafi inventory
echo ">>> Waiting for certificate '${commonname}' to appear in Venafi Cloud inventory..."
for ((i=0;;i++)); do
    if getCertificate; then
        echo "Successfully found certificate in Venafi Cloud."
        exit 0;
    fi;
    echo "Certificate not found yet, retrying in 30 seconds..."
    sleep 30;
done | timeout -v -- 5m cat

echo "!!! Test Failed: Timed out waiting for certificate to appear in Venafi Cloud."
exit 1