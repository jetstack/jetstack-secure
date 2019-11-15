ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

VERSION:=$(shell $(ROOT_DIR)/scripts/getversion.sh)
COMMIT:=$(shell git rev-list -1 HEAD)
DATE:=$(shell date -uR)
GOVERSION:=$(shell go version | awk '{print $$3 " " $$4}')

IMAGE_NAME?=preflight:latest
OVERLAY?=sample

define LDFLAGS
-X "github.com/jetstack/preflight/cmd.PreflightVersion=$(VERSION)" \
-X "github.com/jetstack/preflight/cmd.Platform=$(GOOS)/$(GOARCH)" \
-X "github.com/jetstack/preflight/cmd.Commit=$(COMMIT)" \
-X "github.com/jetstack/preflight/cmd.BuildDate=$(DATE)" \
-X "github.com/jetstack/preflight/cmd.GoVersion=$(GOVERSION)"
endef

GO_BUILD:=go build -ldflags '$(LDFLAGS)'

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
	docker build -t $(IMAGE_NAME) .

push-docker-image:
	docker push $(IMAGE_NAME)
