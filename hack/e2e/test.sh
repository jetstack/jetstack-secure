#!/usr/bin/env bash
#
# Build and install venafi-kubernetes-agent for VenafiConnection based authentication.
# Wait for it to log a message indicating successful data upload.
#
# venafi-kubernetes-agent is packaged using ko and Helm and installed in a Kind cluster.
# A VenafiConnection resource is created which directly loads a bearer token
# from a Kubernetes Secret.
# This is the simplest way of testing the VenafiConnection integration,
# but it does not fully test "secretless" (workload identity federation) authentication.
#
# Prerequisites:
# * ko: https://github.com/ko-build/ko/releases/tag/v0.16.0
# * helm: https://helm.sh/docs/intro/install/
# * kind: https://kubernetes.io/docs/tasks/tools/#kind
# * kubectl: https://kubernetes.io/docs/tasks/tools/#kubectl
# * venctl: https://docs.venafi.cloud/vaas/venctl/t-venctl-install/
# * jq: https://jqlang.github.io/jq/download/
# * step: https://smallstep.com/docs/step-cli/installation/
# * curl: https://www.man7.org/linux/man-pages/man1/curl.1.html
# * envsubst: https://www.man7.org/linux/man-pages/man1/envsubst.1.html
# * gcloud: https://cloud.google.com/sdk/docs/install
# * gke-gcloud-auth-plugin: https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl
# > :warning: If you installed gcloud using snap, you have to install the kubectl plugin using apt:
# > https://github.com/actions/runner-images/issues/6778#issuecomment-1360360603
#
# In case metrics and logs are missing from your cluster, see:
# * https://cloud.google.com/kubernetes-engine/docs/troubleshooting/dashboards#write_permissions

set -o nounset
set -o errexit
set -o pipefail
set -o xtrace
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
root_dir=$(cd "${script_dir}/../.." && pwd)
export TERM=dumb

# Your Venafi Cloud API key.
: ${VEN_API_KEY?}
# Separate API Key for getting a pull secret, if your main venafi cloud tenant
# doesn't allow you to create registry service accounts.
: ${VEN_API_KEY_PULL?}

# The Venafi Cloud zone (application/issuing_template) which will be used by the
# issuer an policy.
: ${VEN_ZONE?}

# The hostname of the Venafi API server.
# US: api.venafi.cloud
# EU: api.venafi.eu
: ${VEN_API_HOST?}

# The base URL of the OCI registry used for Docker images and Helm charts
# E.g. ttl.sh/63773370-0bcf-4ac0-bd42-5515616089ff
: ${OCI_BASE?}

# Required gcloud environment variables
# https://cloud.google.com/sdk/docs/configurations#setting_configuration_properties
: ${CLOUDSDK_CORE_PROJECT?}
: ${CLOUDSDK_COMPUTE_ZONE?}

# The name of the cluster to create
: ${CLUSTER_NAME?}

# IMPORTANT: we pick the first team as the owning team for the registry and
# workload identity service account as it doesn't matter.

version=$(git describe --tags --always --match='v*' --abbrev=14 --dirty)

cd "${script_dir}"

pushd "${root_dir}"
KO_DOCKER_REPO=$OCI_BASE/images/venafi-agent ko build --bare --tags "${version}"
helm package deploy/charts/venafi-kubernetes-agent --version "${version}" --app-version "${version}"
helm push "venafi-kubernetes-agent-${version}.tgz" "oci://${OCI_BASE}/charts"
popd

export USE_GKE_GCLOUD_AUTH_PLUGIN=True
if ! gcloud container clusters get-credentials "${CLUSTER_NAME}"; then
  gcloud container clusters create "${CLUSTER_NAME}" \
    --preemptible \
    --machine-type e2-small \
    --num-nodes 3
fi
kubectl create ns venafi || true

# Pull secret for Venafi OCI registry
if ! kubectl get secret venafi-image-pull-secret -n venafi; then
  venctl iam service-accounts registry create \
    --api-key "${VEN_API_KEY_PULL}" \
    --no-prompts \
    --owning-team "$(curl --fail-with-body -sS "https://${VEN_API_HOST}/v1/teams" -H "tppl-api-key: $VEN_API_KEY_PULL" | jq '.teams[0].id' -r)" \
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

export VENAFI_KUBERNETES_AGENT_CLIENT_ID="not-used-but-required-by-venctl"
venctl components kubernetes apply \
  --cert-manager \
  --venafi-enhanced-issuer \
  --approver-policy-enterprise \
  --venafi-kubernetes-agent \
  --venafi-kubernetes-agent-version "${version}" \
  --venafi-kubernetes-agent-values-files "${script_dir}/values.venafi-kubernetes-agent.yaml" \
  --venafi-kubernetes-agent-custom-image-registry "${OCI_BASE}/images" \
  --venafi-kubernetes-agent-custom-chart-repository "oci://${OCI_BASE}/charts"

kubectl apply -n venafi -f venafi-components.yaml

subject="system:serviceaccount:venafi:venafi-components"
audience="https://${VEN_API_HOST}"
issuerURL="$(kubectl create token -n venafi venafi-components | step crypto jwt inspect --insecure | jq -r '.payload.iss')"
openidDiscoveryURL="${issuerURL}/.well-known/openid-configuration"
jwksURI=$(curl --fail-with-body -sSL ${openidDiscoveryURL} | jq -r '.jwks_uri')

# Create the Venafi agent service account if one does not already exist
while true; do
  tenantID=$(curl --fail-with-body -sSL -H "tppl-api-key: $VEN_API_KEY" https://${VEN_API_HOST}/v1/serviceaccounts \
    | jq -r '.[] | select(.issuerURL==$issuerURL and .subject == $subject) | .companyId' \
      --arg issuerURL "${issuerURL}" \
      --arg subject "${subject}")

  if [[ "${tenantID}" != "" ]]; then
    break
  fi

  jq -n '{
      "name": "venafi-kubernetes-agent-e2e-agent-\($random)",
      "authenticationType": "rsaKeyFederated",
      "scopes": ["kubernetes-discovery-federated", "certificate-issuance"],
      "subject": $subject,
      "audience": $audience,
      "issuerURL": $issuerURL,
      "jwksURI": $jwksURI,
      "applications": [$applications.applications[].id],
      "owner": $owningTeamID
    }' \
    --arg random "${RANDOM}" \
    --arg subject "${subject}" \
    --arg audience "${audience}" \
    --arg issuerURL "${issuerURL}" \
    --arg jwksURI "${jwksURI}" \
    --arg owningTeamID "$(curl --fail-with-body -sS "https://${VEN_API_HOST}/v1/teams" -H "tppl-api-key: $VEN_API_KEY" | jq '.teams[0].id' -r)" \
    --argjson applications "$(curl https://${VEN_API_HOST}/outagedetection/v1/applications --fail-with-body -sSL -H tppl-api-key:\ ${VEN_API_KEY})" \
    | curl https://${VEN_API_HOST}/v1/serviceaccounts \
      -H "tppl-api-key: $VEN_API_KEY" \
      --fail-with-body \
      -sSL --json @-
done

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

envsubst <application-team-1.yaml | kubectl apply -f -
kubectl -n team-1 wait certificate app-0 --for=condition=Ready

# Wait for log message indicating success.
# Filter out distracting data gatherer errors and warnings.
# Show other useful log messages on stderr.
kubectl logs deployments/venafi-kubernetes-agent \
  --follow \
  --namespace venafi \
  | tee >(grep -v -e "reflector\.go" -e "datagatherer" -e "data gatherer" >/dev/stderr) \
  | grep -q "Data sent successfully"
