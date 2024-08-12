# This project isn't fully migrated to use makefile-modules and klone. For now,
# we just use the "tools" module. We may fully migrate later on.

ROOT_DIR:=$(shell dirname $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST)))))
VERSION:=$(shell $(ROOT_DIR)/hack/getversion)
COMMIT:=$(shell git rev-list -1 HEAD)
DATE:=$(shell date -uR)
GOVERSION:=$(shell go version | awk '{print $$3 " " $$4}')
GOOS:=$(shell go env GOOS)
GOARCH:=$(shell go env GOARCH)

BIN_NAME:=preflight

DOCKER_IMAGE?=quay.io/jetstack/preflight
DOCKER_IMAGE_TAG?=$(DOCKER_IMAGE):$(COMMIT)

# BUILD_IN decides if the binaries will be built in `docker` or in the `host`.
BUILD_IN?=docker

# OAuth2 config for the agent to work with platform.jetstack.io
OAUTH_CLIENT_ID?=k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo
OAUTH_CLIENT_SECRET?=f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa
OAUTH_AUTH_SERVER_DOMAIN?=auth.jetstack.io

define LDFLAGS
-X "github.com/jetstack/preflight/pkg/version.PreflightVersion=$(VERSION)" \
-X "github.com/jetstack/preflight/pkg/version.Platform=$(GOOS)/$(GOARCH)" \
-X "github.com/jetstack/preflight/pkg/version.Commit=$(COMMIT)" \
-X "github.com/jetstack/preflight/pkg/version.BuildDate=$(DATE)" \
-X "github.com/jetstack/preflight/pkg/version.GoVersion=$(GOVERSION)" \
-X "github.com/jetstack/preflight/pkg/client.ClientID=$(OAUTH_CLIENT_ID)" \
-X "github.com/jetstack/preflight/pkg/client.ClientSecret=$(OAUTH_CLIENT_SECRET)" \
-X "github.com/jetstack/preflight/pkg/client.AuthServerDomain=$(OAUTH_AUTH_SERVER_DOMAIN)"
endef

GO_BUILD:=CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)'
GO_INSTALL:=CGO_ENABLED=0 go install -ldflags '$(LDFLAGS)'

export GO111MODULE=on

# Golang cli

.PHONY: build
build:
	cd $(ROOT_DIR) && $(GO_BUILD) -o builds/preflight .

install:
	cd $(ROOT_DIR) && $(GO_INSTALL)

test:
	cd $(ROOT_DIR) && go test ./...

vet:
	cd $(ROOT_DIR) && go vet ./...


.PHONY: ./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME)
$(ROOT_DIR)/builds/$(GOOS)/$(GOARCH)/$(BIN_NAME):
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o ./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME) .
.PHONY: ./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME)
$(ROOT_DIR)/builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME):
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(GO_BUILD) -o ./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME) .

build-all-platforms: build-all-platforms-in-$(BUILD_IN)

build-all-platforms-in-host:
	$(MAKE) GOOS=linux   GOARCH=amd64       ./builds/linux/amd64/$(BIN_NAME)
	$(MAKE) GOOS=linux   GOARCH=arm64       ./builds/linux/arm64/$(BIN_NAME)
	$(MAKE) GOOS=linux   GOARCH=arm GOARM=7 ./builds/linux/arm/v7/$(BIN_NAME)
	$(MAKE) GOOS=darwin  GOARCH=amd64       ./builds/darwin/amd64/$(BIN_NAME)
	$(MAKE) GOOS=windows GOARCH=amd64       ./builds/windows/amd64/$(BIN_NAME)

build-all-platforms-in-docker:
	rm -rf $(ROOT_DIR)/builds
	docker build --rm -t preflight-bin --load -f $(ROOT_DIR)/builder.dockerfile \
		--build-arg oauth_client_id=$(OAUTH_CLIENT_ID) \
		--build-arg oauth_client_secret=$(OAUTH_CLIENT_SECRET) \
		--build-arg oauth_auth_server_domain=$(OAUTH_AUTH_SERVER_DOMAIN) \
		.
	docker create --rm --name=preflight-bin-container preflight-bin
	docker cp preflight-bin-container:/go/github.com/jetstack/preflight/builds $(ROOT_DIR)/builds
	docker rm preflight-bin-container
	docker rmi preflight-bin

# Docker image
PLATFORMS?=linux/arm/v7,linux/arm64/v8,linux/amd64
BUILDX_EXTRA_ARGS?=

push_buildx_args=--push $(BUILDX_EXTRA_ARGS)
push-canary_buildx_args=--tag $(DOCKER_IMAGE):canary --push $(BUILDX_EXTRA_ARGS)
build_buildx_args=$(BUILDX_EXTRA_ARGS)

.PHONY: _docker-%
_docker-%: build-all-platforms
	docker buildx build --platform $(PLATFORMS) \
	--tag $(DOCKER_IMAGE):$(VERSION) \
	--tag $(DOCKER_IMAGE):latest \
	--tag $(DOCKER_IMAGE):canary \
	$($*_buildx_args) \
	.

build-docker-image: _docker-build
push-docker-image: _docker-push

export COSIGN_REPOSITORY?=ghcr.io/jetstack/jetstack-secure/cosign
export COSIGN_EXPERIMENTAL=1

.PHONY: sign-docker-image
sign-docker-image:
	@cosign sign -y $(DOCKER_IMAGE):$(VERSION)

.PHONY: sbom-docker-image
sbom-docker-image:
	@syft $(DOCKER_IMAGE):$(VERSION) -o cyclonedx > bom.xml
	@cosign attach sbom --sbom bom.xml --type cyclonedx $(DOCKER_IMAGE):$(VERSION)
	@cosign sign -y --attachment sbom $(DOCKER_IMAGE):$(VERSION)

.PHONY: attest-docker-image
attest-docker-image:
	@cosign attest -y --type slsaprovenance --predicate predicate.json $(DOCKER_IMAGE):$(VERSION)

# A pre-commit hook is configured on this repository and can be installed using https://pre-commit.com/#3-install-the-git-hook-scripts
# This target can be used instead if the pre-commit hook is not desired
.PHONY: update-helm-docs
update-helm-docs:
	go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.11.0
	helm-docs --chart-search-root=deploy/charts/

# CI

export PATH:=$(GOPATH)/bin:$(PATH)

ci-deps:
	echo "ci-deps is going to be disabled. We are adopting Github actions"
	go install golang.org/x/lint/golint

ci-test: ci-deps test lint

ci-build: ci-test build build-docker-image build-all-platforms bundle-all-platforms push-docker-image-canary
	echo "ci-build is going to be disabled. We are adopting Github actions"

ci-publish: ci-build push-docker-image
	echo "ci-publish is going to be disabled. We are adopting Github actions"
