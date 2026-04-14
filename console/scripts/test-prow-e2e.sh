#!/usr/bin/env bash

set -exuo pipefail

ARTIFACT_DIR=${ARTIFACT_DIR:=/tmp/artifacts}
SCREENSHOTS_DIR=integration-tests/screenshots
INSTALLER_DIR=${INSTALLER_DIR:=${ARTIFACT_DIR}/installer}

function copyArtifacts {
  if [ -d "$ARTIFACT_DIR" ] && [ -d "$SCREENSHOTS_DIR" ]; then
    if [[ -z "$(ls -A -- "$SCREENSHOTS_DIR")" ]]; then
      echo "No artifacts were copied."
    else
      echo "Copying artifacts from $(pwd)..."
      cp -r "$SCREENSHOTS_DIR" "${ARTIFACT_DIR}/screenshots"
    fi
  fi
}

trap copyArtifacts EXIT


# don't log kubeadmin-password
set +x
if [ -z "${KUBEADMIN_PASSWORD:-}" ]; then
  echo "ERROR: KUBEADMIN_PASSWORD is not set" >&2
  exit 1
fi
BRIDGE_KUBEADMIN_PASSWORD="${KUBEADMIN_PASSWORD}"
export BRIDGE_KUBEADMIN_PASSWORD
set -x
BRIDGE_BASE_ADDRESS="$(oc get consoles.config.openshift.io cluster -o jsonpath='{.status.consoleURL}')"
export BRIDGE_BASE_ADDRESS

echo "Install dependencies"
if [ ! -d node_modules ]; then
  yarn install
fi

echo "Runs Cypress tests in headless mode"
yarn run test-cypress-headless