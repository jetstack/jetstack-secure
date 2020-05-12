ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

VERSION:=$(shell $(ROOT_DIR)/hack/getversion)
COMMIT:=$(shell git rev-list -1 HEAD)
DATE:=$(shell date -uR)
GOVERSION:=$(shell go version | awk '{print $$3 " " $$4}')
GOOS:=$(shell go env GOOS)
GOARCH:=$(shell go env GOARCH)

DOCKER_IMAGE?=quay.io/jetstack/preflight
DOCKER_IMAGE_TAG?=$(DOCKER_IMAGE):$(VERSION)

# OAuth2 config for the agent to work with preflight.jetstack.io
OAUTH_CLIENT_ID?="k3TrDbfLhCgnpAbOiiT2kIE1AbovKzjo"
OAUTH_CLIENT_SECRET?="f39w_3KT9Vp0VhzcPzvh-uVbudzqCFmHER3Huj0dvHgJwVrjxsoOQPIw_1SDiCfa"
OAUTH_AUTH_SERVER_DOMAIN?="jetstack-prod.eu.auth0.com"

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

GO_BUILD:=go build -ldflags '$(LDFLAGS)'
GO_INSTALL:=go install -ldflags '$(LDFLAGS)'

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

.PHONY: ./builds/preflight-$(GOOS)-$(GOARCH)
./builds/preflight-$(GOOS)-$(GOARCH):
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO_BUILD) -o ./builds/preflight-$(GOOS)-$(GOARCH) .

build-all-platforms:
	$(MAKE) GOOS=linux   GOARCH=amd64 ./builds/preflight-linux-amd64
	$(MAKE) GOOS=darwin  GOARCH=amd64 ./builds/preflight-darwin-amd64
	$(MAKE) GOOS=windows GOARCH=amd64 ./builds/preflight-windows-amd64

# Bundles

./bundles/preflight-bundle-$(GOOS)-$(GOARCH).tgz: ./builds/preflight-$(GOOS)-$(GOARCH)
	cd $(ROOT_DIR) && \
	mkdir -p ./bundles && \
	tar --transform "s/builds\/preflight-$(GOOS)-$(GOARCH)/preflight/" -rvf $@.tmp $< && \
	gzip < $@.tmp > $@ && \
	rm $@.tmp

bundle-all-platforms:
	$(MAKE) GOOS=linux   GOARCH=amd64 ./bundles/preflight-bundle-linux-amd64.tgz
	$(MAKE) GOOS=darwin  GOARCH=amd64 ./bundles/preflight-bundle-darwin-amd64.tgz
	$(MAKE) GOOS=windows GOARCH=amd64 ./bundles/preflight-bundle-windows-amd64.tgz

# Docker image

build-docker-image:
	docker build --tag $(DOCKER_IMAGE_TAG) .

push-docker-image:
	docker tag $(DOCKER_IMAGE_TAG) $(DOCKER_IMAGE):latest
	docker push $(DOCKER_IMAGE_TAG)
	docker push $(DOCKER_IMAGE):latest

push-docker-image-canary:
	docker tag $(DOCKER_IMAGE_TAG) $(DOCKER_IMAGE):canary
	docker push $(DOCKER_IMAGE_TAG)
	docker push $(DOCKER_IMAGE):canary

# CI

export PATH:=$(GOPATH)/bin:$(PATH)

ci-deps:
	go install golang.org/x/lint/golint

ci-test: ci-deps test lint

ci-build: ci-test build build-docker-image build-all-platforms bundle-all-platforms push-docker-image-canary

ci-publish: ci-build push-docker-image
