ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

VERSION:=$(shell $(ROOT_DIR)/hack/getversion)
COMMIT:=$(shell git rev-list -1 HEAD)
DATE:=$(shell date -uR)
GOVERSION:=$(shell go version | awk '{print $$3 " " $$4}')

DOCKER_IMAGE?=quay.io/jetstack/preflight
DOCKER_IMAGE_TAG?=$(DOCKER_IMAGE):$(VERSION)

define LDFLAGS
-X "github.com/jetstack/preflight/cmd.PreflightVersion=$(VERSION)" \
-X "github.com/jetstack/preflight/cmd.Platform=$(GOOS)/$(GOARCH)" \
-X "github.com/jetstack/preflight/cmd.Commit=$(COMMIT)" \
-X "github.com/jetstack/preflight/cmd.BuildDate=$(DATE)" \
-X "github.com/jetstack/preflight/cmd.GoVersion=$(GOVERSION)"
endef

GO_BUILD:=go build -ldflags '$(LDFLAGS)'

export GO111MODULE=on

.PHONY: build

build:
	cd $(ROOT_DIR) && $(GO_BUILD) -o builds/preflight .

test:
	cd $(ROOT_DIR) && go test ./...

vet:
	cd $(ROOT_DIR) && go vet ./...

lint: vet
	cd $(ROOT_DIR) && golint

clean:
	cd $(ROOT_DIR) && rm -rf ./builds

build-docker-image:
	docker build --tag $(DOCKER_IMAGE_TAG) .

push-docker-image:
	docker tag $(DOCKER_IMAGE_TAG) $(DOCKER_IMAGE):latest
	docker push $(DOCKER_IMAGE_TAG)
	docker push $(DOCKER_IMAGE):latest

ci-test: test lint

ci-build: ci-test build build-docker-image

ci-publish: ci-build push-docker-image
