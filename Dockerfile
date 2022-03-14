FROM registry.access.redhat.com/ubi8/ubi-minimal
RUN microdnf install git golang -y && microdnf clean all
RUN go version


# Build the manager binary
#FROM golang:1.17 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN go mod download
# Or, we could not do that

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY version/ version/
COPY controllers/ controllers/
COPY vendor/ vendor/
COPY hack/ hack/
COPY .git/ .git/

# Build
RUN hack/build.sh

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
