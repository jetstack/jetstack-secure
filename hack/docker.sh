#!/bin/bash

# Script that builds and pushes docker images for multiple architectures. If you want to include a new architecture,
# ensure that it is compiled in the build.sh script, then add your OS/ARCH combination to the PLATFORMS variable.

PLATFORMS=linux/arm/v7,linux/arm64/v8,linux/amd64
DOCKER_IMAGE=quay.io/jetstack/preflight

# Uses the git tag or commit SHA.
VERSION="$(bash ./hack/version.sh)"

docker buildx build \
  --platform ${PLATFORMS} \
  --tag "${DOCKER_IMAGE}:${VERSION}" \
  --tag "${DOCKER_IMAGE}:latest" \
  --push \
  .
