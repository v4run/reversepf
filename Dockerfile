# syntax=docker/dockerfile:1

################################################################################
# Create a stage for building the application.
ARG GO_VERSION=1.21.4
FROM golang:${GO_VERSION}-alpine3.18 AS build
WORKDIR /src

ARG APP_VERSION=development
ARG COMMIT_HASH
ARG BUILD_DATE

# Download dependencies as a separate step to take advantage of Docker's caching.
# Leverage a cache mount to /go/pkg/mod/ to speed up subsequent builds.
# Leverage bind mounts to go.sum and go.mod to avoid having to copy them into
# the container.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

# Build the application.
# Leverage a cache mount to /go/pkg/mod/ to speed up subsequent builds.
# Leverage a bind mount to the current directory to avoid having to copy the
# source code into the container.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build -o /bin/reversepf -ldflags="-X 'github.com/v4run/reversepf/version.Version=${APP_VERSION}' -X 'github.com/v4run/reversepf/version.BuildDate=${BUILD_DATE}' -X 'github.com/v4run/reversepf/version.CommitHash=${COMMIT_HASH}' " .

FROM alpine:3.18 AS final

ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser
USER appuser

# Copy the executable from the "build" stage.
COPY --from=build /bin/reversepf /bin/

ENTRYPOINT [ "/bin/reversepf" ]
