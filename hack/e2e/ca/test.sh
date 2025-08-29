#!/usr/bin/env bash
#
set -o nounset
set -o errexit
set -o pipefail

# CyberArk API configuration
: ${ARK_USERNAME?}
: ${ARK_SECRET?}
: ${ARK_SUBDOMAIN?}
: ${ARK_DISCOVERY_API?}

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
     oci_preflight_image_name=$OCI_BASE/images/cyberark-disco-agent \
     helm_chart_image_name=$OCI_BASE/charts/venafi-kubernetes-agent \
     GITHUB_OUTPUT="${tmp_dir}/release.env"
source release.env

kind create cluster || true
kubectl create ns "$k8s_namespace" || true

kubectl delete secret agent-credentials --namespace "$k8s_namespace" --ignore-not-found
kubectl create secret generic agent-credentials \
        --namespace "$k8s_namespace" \
        --from-literal=ARK_USERNAME=$ARK_USERNAME \
        --from-literal=ARK_SECRET=$ARK_SECRET \
        --from-literal=ARK_SUBDOMAIN=$ARK_SUBDOMAIN \
        --from-literal=ARK_DISCOVERY_API=$ARK_DISCOVERY_API

helm upgrade agent "${root_dir}/deploy/charts/cyberark-disco-agent" \
     --install \
     --create-namespace \
     --namespace "$k8s_namespace" \
     --set fullnameOverride=disco-agent \
     --set "image.repository=${RELEASE_OCI_PREFLIGHT_IMAGE}" \
     --set "image.tag=${RELEASE_OCI_PREFLIGHT_TAG}" \
     --set-json "podLabels={\"test\": \"${RANDOM}\"}" \
     --values "${script_dir}/values.agent.yaml"
