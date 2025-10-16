ARG GO_VERSION=1.24.6
FROM registry.access.redhat.com/ubi9/go-toolset:${GO_VERSION} AS builder
ARG GOARCH=amd64

# Build the manager binary

WORKDIR /workspace

# Copy the go source
COPY go.mod go.mod
COPY go.sum go.sum

# Copy the go sources
COPY vendor/ vendor/
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY version/ version/
COPY internal/controller/ internal/controller/
COPY hack/ hack/
COPY .git/ .git/
COPY LICENSE /licenses/
COPY Makefile .

# Build
USER root
RUN --mount=type=secret,id=apikey GOARCH=${GOARCH} make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot

# UBI is larger (158Mb vs. 56Mb) but approved by RH 
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /licenses/ /licenses/
USER 65532:65532

ENTRYPOINT ["/manager"]
