.PHONY: build image load-image push-image install

APP_VERSION=v0.0.2

build:
	go build -trimpath -ldflags="-s -w -X 'github.com/v4run/reversepf/version.Version=${APP_VERSION}' -X 'github.com/v4run/reversepf/version.BuildDate=$(shell date)' -X 'github.com/v4run/reversepf/version.CommitHash=$(shell git rev-parse HEAD)'"

install:
	go install -trimpath -ldflags="-s -w -X 'github.com/v4run/reversepf/version.Version=${APP_VERSION}' -X 'github.com/v4run/reversepf/version.BuildDate=$(shell date)' -X 'github.com/v4run/reversepf/version.CommitHash=$(shell git rev-parse HEAD)'"

image:
	docker image build\
		--build-arg "APP_VERSION=${APP_VERSION}"\
		--build-arg "COMMIT_HASH=$(shell git rev-parse HEAD)"\
		--build-arg "BUILD_DATE=$(shell date)"\
		-t v4run/reversepf:"${APP_VERSION}" .

load-image:
	# Load the image to local kind k8s cluster
	kind load docker-image v4run/reversepf:${APP_VERSION}

push-image:
	docker buildx build\
		--build-arg "APP_VERSION=${APP_VERSION}"\
		--build-arg "COMMIT_HASH=$(shell git rev-parse HEAD)"\
		--build-arg "BUILD_DATE=$(shell date)"\
		--push --platform linux/arm/v7,linux/arm64/v8,linux/amd64 -t v4run/reversepf:${APP_VERSION} .
