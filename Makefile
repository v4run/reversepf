.PHONY: build image

APP_VERSION=v0.0.1

build:
	go build -trimpath -ldflags="-s -w -X 'github.com/v4run/reversepf/version.Version=${APP_VERSION}' -X 'github.com/v4run/reversepf/version.BuildDate=${BUILD_DATE}' -X 'github.com/v4run/reversepf/version.CommitHash=${COMMIT_HASH}'"

image:
	docker image build\
		--build-arg "APP_VERSION=${APP_VERSION}"\
		--build-arg "COMMIT_HASH=$(shell git rev-parse HEAD)"\
		--build-arg "BUILD_DATE=$(shell date)"\
		-t v4run/reversepf:"${APP_VERSION}" .

load-image:
	# Load the image to local kind k8s cluster
	kind load docker-image v4run/reversepf:${APP_VERSION}
