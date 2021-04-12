ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

VERSION:=$(shell $(ROOT_DIR)/hack/getversion)
COMMIT:=$(shell git rev-list -1 HEAD)
DATE:=$(shell date -uR)
GOVERSION:=$(shell go version | awk '{print $$3 " " $$4}')
GOOS:=$(shell go env GOOS)
GOARCH:=$(shell go env GOARCH)

BIN_NAME:=preflight

DOCKER_IMAGE?=quay.io/jetstack/preflight
DOCKER_IMAGE_TAG?=$(DOCKER_IMAGE):$(VERSION)

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

clean:
	cd $(ROOT_DIR) && rm -rf ./builds ./bundles

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

lint: vet
	cd $(ROOT_DIR) && golint


.PHONY: ./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME)
./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME):
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o ./builds/$(GOOS)/$(GOARCH)/$(BIN_NAME) .
.PHONY: ./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME)
./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME):
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) $(GO_BUILD) -o ./builds/$(GOOS)/$(GOARCH)/v$(GOARM)/$(BIN_NAME) .

build-all-platforms: build-all-platforms-in-$(BUILD_IN)

build-all-platforms-in-host:
	$(MAKE) GOOS=linux   GOARCH=amd64       ./builds/linux/amd64/$(BIN_NAME)
	$(MAKE) GOOS=linux   GOARCH=arm64       ./builds/linux/arm64/$(BIN_NAME)
	$(MAKE) GOOS=linux   GOARCH=arm GOARM=7 ./builds/linux/arm/v7/$(BIN_NAME)
	$(MAKE) GOOS=darwin  GOARCH=amd64       ./builds/darwin/amd64/$(BIN_NAME)
	$(MAKE) GOOS=windows GOARCH=amd64       ./builds/windows/amd64/$(BIN_NAME)

build-all-platforms-in-docker:
	rm -rf ./builds
	docker build --rm -t preflight-bin -f ./builder.dockerfile \
		--build-arg oauth_client_id=$(OAUTH_CLIENT_ID) \
		--build-arg oauth_client_secret=$(OAUTH_CLIENT_SECRET) \
		--build-arg oauth_auth_server_domain=$(OAUTH_AUTH_SERVER_DOMAIN) \
		.
	docker create --rm --name=preflight-bin-container preflight-bin
	docker cp preflight-bin-container:/go/github.com/jetstack/preflight/builds ./builds
	docker rm preflight-bin-container
	docker rmi preflight-bin

# Docker image
PLATFORMS?=linux/arm/v7,linux/arm64/v8,linux/amd64
BUILDX_EXTRA_ARGS?=

push_buildx_args=--tag $(DOCKER_IMAGE):latest --push $(BUILDX_EXTRA_ARGS)
push-canary_buildx_args=--tag $(DOCKER_IMAGE):canary --push $(BUILDX_EXTRA_ARGS)
build_buildx_args=$(BUILDX_EXTRA_ARGS)

.PHONY: _docker-%
_docker-%: build-all-platforms
	docker buildx build --platform $(PLATFORMS) \
	--tag $(DOCKER_IMAGE_TAG) \
	$($*_buildx_args) \
	.

build-docker-image: _docker-build
push-docker-image: _docker-push
push-docker-image-canary: _docker-push-canary

# CI

export PATH:=$(GOPATH)/bin:$(PATH)

ci-deps:
	go install golang.org/x/lint/golint

ci-test: ci-deps test lint

ci-build: ci-test build build-docker-image build-all-platforms bundle-all-platforms push-docker-image-canary

ci-publish: ci-build push-docker-image
