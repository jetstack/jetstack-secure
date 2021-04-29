#!/bin/bash

# Script to compile the agent binary for different architectures. To add a new architecture, create a new function named
# compile_<GOOS>_<GOARCH. Within that function, set the GOOS and GOARCH variables for the architecture you want, then
# call the compile function. Binaries are output in ./builds/<GOOS>/<GOARCH>/

BIN_NAME=preflight

# Build information that is compiled into the binaries.
COMMIT=$(git rev-list -1 HEAD)
DATE="$(date -uR)"
VERSION="$(bash ./hack/version.sh)"

# OAuth2 config for the agent to work with platform.jetstack.io
OAUTH_CLIENT_ID=k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo
OAUTH_CLIENT_SECRET=f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa
OAUTH_AUTH_SERVER_DOMAIN=auth.jetstack.io

GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
GOARM=$(go env GOARM)

function compile_all() {
  compile_linux_amd64
  compile_linux_arm64
  compile_linux_armv7
  compile_darwin_amd64
  compile_windows_amd64
}

function compile_windows_amd64() {
  GOOS=windows
  GOARCH=amd64

  compile
}

function compile_darwin_amd64() {
  GOOS=darwin
  GOARCH=amd64

  compile
}

function compile_linux_armv7() {
  GOOS=linux
  GOARCH=arm
  GOARM=7

  compile
}

function compile_linux_arm64() {
  GOOS=linux
  GOARCH=arm64

  compile
}

function compile_linux_amd64() {
  GOOS=linux
  GOARCH=amd64

  compile
}

function compile() {
  OUTPUT=./builds/${GOOS}/${GOARCH}/${BIN_NAME}

  # If GOARM has been set, add an extra directory.
  if [ -n "$GOARM" ]; then
    OUTPUT=./builds/${GOOS}/${GOARCH}/v${GOARM}/${BIN_NAME}
  fi

  GOOS=${GOOS} GOARCH=${GOARCH} GOARM=${GOARM} CGO_ENABLED=0 go build -ldflags \
  "-w -s "\
"-X 'github.com/jetstack/preflight/pkg/version.PreflightVersion=${VERSION}'"\
"-X 'github.com/jetstack/preflight/pkg/version.Platform=${GOOS}/${GOARCH}'"\
"-X 'github.com/jetstack/preflight/pkg/version.Commit=${COMMIT}'"\
"-X 'github.com/jetstack/preflight/pkg/version.BuildDate=${DATE}'"\
"-X 'github.com/jetstack/preflight/pkg/version.GoVersion=${GOVERSION}'"\
"-X 'github.com/jetstack/preflight/pkg/client.ClientID=${OAUTH_CLIENT_ID}'"\
"-X 'github.com/jetstack/preflight/pkg/client.ClientSecret=${OAUTH_CLIENT_SECRET}'"\
"-X 'github.com/jetstack/preflight/pkg/client.AuthServerDomain=${OAUTH_AUTH_SERVER_DOMAIN}'" \
  -o "${OUTPUT}"

  GOARM=""
}

"$@"
