FROM registry.access.redhat.com/ubi9/ubi-minimal AS builder
RUN microdnf install git-core golang -y && microdnf clean all

# ubi8 now has golang 1.19 so we use that.
# If a different version is needed, uncomment the following
# ENV GO_VERSION=1.19
# RUN go install golang.org/dl/go${GO_VERSION}@latest
# RUN ~/go/bin/go${GO_VERSION} download
# RUN /bin/cp -f ~/go/bin/go${GO_VERSION} /usr/bin/go

RUN go version

# Build the manager binary

WORKDIR /workspace

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN go mod download
# Or, we could not do that

# Copy the go source
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ vendor/
COPY main.go main.go
COPY api/ api/
COPY version/ version/
COPY controllers/  controllers/ 
COPY hack/ hack/
COPY .git/ .git/

# Build
RUN --mount=type=secret,id=apikey hack/build.sh

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot

# UBI is larger (158Mb vs. 56Mb) but approved by RH
FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /
RUN microdnf install git-core -y && microdnf clean all
RUN mkdir -p /.config/helm && mkdir /.cache && chmod 770 /.config/helm && chmod 770 /.cache
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
