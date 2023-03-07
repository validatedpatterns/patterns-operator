#!/bin/bash
set -ex

if [[ -z "$GOOS" ]] ; then
    >&2 echo "GOOS must be set! Example: GOOS=linux GOARCH=amd64 ./hack/build.sh"
    exit 1
elif [[ -z "$GOARCH" ]] ; then
    >&2 echo "GOARCH must be set! Example: GOOS=linux GOARCH=amd64 ./hack/build.sh"
    exit 1
fi

GIT_VERSION=$(git describe --always --tags || true)
VERSION=${CI_UPSTREAM_VERSION:-${GIT_VERSION}}
GIT_COMMIT=$(git rev-list -1 HEAD || true)
COMMIT=${CI_UPSTREAM_COMMIT:-${GIT_COMMIT}}
BUILD_DATE=$(date --utc -Iseconds)

mkdir -p _out

LDFLAGS="-s -w "
REPO="github.com/hybrid-cloud-patterns/patterns-operator"
LDFLAGS+="-X $REPO/version.Version=${VERSION} "
LDFLAGS+="-X $REPO/version.GitCommit=${COMMIT} "
LDFLAGS+="-X $REPO/version.BuildDate=${BUILD_DATE} "

EXTRA=""
case $1 in
    "run") EXTRA="run";;
    *)	EXTRA="build -o manager";;
esac
GOFLAGS=-mod=vendor CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go $EXTRA -ldflags="${LDFLAGS}" main.go
