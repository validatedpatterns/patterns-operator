FROM quay.io/centos/centos:stream9 AS builder
RUN dnf install git-core golang jq -y && dnf clean all

# Build the manager binary

WORKDIR /workspace

# Copy the go source
COPY go.mod go.mod
COPY go.sum go.sum

# use latest Go z release
ENV GOTOOLCHAIN=auto

# Ensure correct Go version
RUN export GO_VERSION=$(grep -E "go [[:digit:]]\.[[:digit:]][[:digit:]]" go.mod | awk '{print $2}') && \
    export GO_FULLVER=$(curl -LsS 'https://golang.org/dl/?mode=json' | jq -r '.[] | select(.version | startswith("go'${GO_VERSION}'")) | .version' | sort -V | tail -n1) && \
    go install golang.org/dl/${GO_FULLVER}@latest && \
    ~/go/bin/${GO_FULLVER} download && \
    /bin/cp -f ~/go/bin/${GO_FULLVER} /usr/bin/go && \
    go version

# Copy the go sources
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
# 20240418 - bandini - switching to ubi-micro
FROM registry.access.redhat.com/ubi9/ubi-micro:latest
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
