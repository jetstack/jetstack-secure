ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

export GO111MODULE=on

clean:
	cd $(ROOT_DIR) && rm -rf ./builds ./bundles

.PHONY: build
build:
	./hack/build.sh

test:
	cd $(ROOT_DIR) && go test ./...

vet:
	cd $(ROOT_DIR) && go vet ./...

lint: vet
	cd $(ROOT_DIR) && golint

docker:
	./hack/docker.sh

docker-test:
	docker buildx build --platform linux/amd64 -t preflight .
