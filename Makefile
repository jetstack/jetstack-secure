ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

export GO111MODULE=on

clean:
	cd $(ROOT_DIR) && rm -rf ./builds ./bundles

build:
	./hack/build.sh compile

multi-arch:
	./hack/build.sh compile_all

test:
	cd $(ROOT_DIR) && go test ./...

vet:
	cd $(ROOT_DIR) && go vet ./...

lint: vet
	cd $(ROOT_DIR) && golint

docker:
	./hack/docker.sh

docker-test: build
	docker buildx build --platform linux/amd64 -t preflight .
