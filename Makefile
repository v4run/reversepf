.PHONY: build

APP_VERSION=v0.0.1

image:
	docker image build\
		--build-arg "APP_VERSION=${APP_VERSION}"\
		--build-arg "COMMIT_HASH=$(shell git rev-parse HEAD)"\
		--build-arg "BUILD_DATE=$(shell date)"\
		-t v4run/reversepf:"${APP_VERSION}" .

