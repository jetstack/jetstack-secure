#!/usr/bin/env bash
#
set -o nounset
set -o errexit
set -o pipefail

# CyberArk API configuration
: ${ARK_USERNAME?}
: ${ARK_SECRET?}
: ${ARK_PLATFORM_DOMAIN?}
: ${ARK_SUBDOMAIN?}

# The base URL of the OCI registry used for Docker images and Helm charts
# E.g. ttl.sh/6ee49a01-c8ba-493e-bae9-4d8567574b56
: ${OCI_BASE?}

k8s_namespace=cyberark

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
root_dir=$(cd "${script_dir}/../../.." && pwd)
export TERM=dumb

tmp_dir="$(mktemp -d /tmp/jetstack-secure.XXXXX)"

pushd "${tmp_dir}"
> release.env
make -C "$root_dir" release \
     OCI_SIGN_ON_PUSH=false \
     oci_platforms=linux/amd64 \
     oci_preflight_image_name=$OCI_BASE/images/venafi-agent \
     helm_chart_image_name=$OCI_BASE/charts/venafi-kubernetes-agent \
     GITHUB_OUTPUT="${tmp_dir}/release.env"
source release.env

kind create cluster || true
kubectl create ns "$k8s_namespace" || true

kubectl create secret generic agent-credentials \
        --namespace "$k8s_namespace" \
        --from-literal=ARK_USERNAME=$ARK_USERNAME \
        --from-literal=ARK_SECRET=$ARK_SECRET \
        --from-literal=ARK_PLATFORM_DOMAIN=$ARK_PLATFORM_DOMAIN \
        --from-literal=ARK_SUBDOMAIN=$ARK_SUBDOMAIN

helm upgrade agent "oci://${OCI_BASE}/charts/venafi-kubernetes-agent" \
     --install \
     --create-namespace \
     --namespace "$k8s_namespace" \
     --version "${RELEASE_HELM_CHART_VERSION}" \
     --set fullnameOverride=agent \
     --set "image.repository=${OCI_BASE}/images/venafi-agent" \
     --values "${script_dir}/values.agent.yaml"

kubectl scale -n "$k8s_namespace" deployment agent  --replicas=0
kubectl get cm -n "$k8s_namespace" agent-config -o jsonpath={.data.config\\.yaml} > config.original.yaml
yq eval-all '. as $item ireduce ({}; . * $item)' config.original.yaml "${script_dir}/config.yaml" > config.yaml
kubectl delete cm -n "$k8s_namespace" agent-config
kubectl create cm -n "$k8s_namespace" agent-config --from-file=config.yaml
kubectl scale -n "$k8s_namespace" deployment agent  --replicas=1
