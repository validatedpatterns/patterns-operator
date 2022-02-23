#!/bin/bash
set -ex

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
GOFLAGS=-mod=vendor CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go "${EXTRA}" -ldflags="${LDFLAGS}" main.go
