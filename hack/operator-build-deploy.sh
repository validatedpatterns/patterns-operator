#!/bin/bash
set -e -o pipefail

CATALOGSOURCE="test-pattern-operator"
DEFAULT_NS="patterns-operator"
OPERATOR="patterns-operator"
VERSION="${VERSION:-6.6.6}"
UPLOADREGISTRY="${UPLOADREGISTRY:-kuemper.int.rhx/bandini}"

wait_for_resource() {
    local resource_type=$1  # Either "packagemanifest", "operator", or "csv"
    local name=$2           # Name of the resource (e.g., Operator or CSV)
    local namespace=$3      # Namespace (optional, required for CSV and Operator)
    local label=$4          # Label selector (only for packagemanifests)

    echo "⏳ Waiting for $resource_type: $name"
    while true; do
        set +e
        if [[ "$resource_type" == "packagemanifest" ]]; then
            oc get -n openshift-marketplace packagemanifests -l "catalog=${label}" --field-selector "metadata.name=${name}" &> /dev/null
        elif [[ "$resource_type" == "operator" ]]; then
            oc get operators.operators.coreos.com "${name}.${namespace}" &> /dev/null
        elif [[ "$resource_type" == "csv" ]]; then
            STATUS=$(oc get csv "$name" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null)
            if [[ "$STATUS" == "Succeeded" ]]; then
                echo "✅ Operator installation completed successfully!"
                break
            fi
            echo "⏳ Operator installation in progress... (Current status: ${STATUS:-Not Found})"
        else
            echo "❌ Unknown resource type: $resource_type"
            return 1
        fi
        ret=$?
        set -e

        if [[ $ret -eq 0 && "$resource_type" != "csv" ]]; then
            echo "✅ $resource_type: $name is available!"
            break
        fi

        sleep 10
    done
}

apply_subscription() {
    oc delete -n ${NS} subscription/${OPERATOR} || /bin/true
    oc delete catalogsource/${CATALOGSOURCE} || /bin/true
    oc apply -f - <<EOF
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: ${OPERATOR}
      namespace: ${NS}
    spec:
      channel: fast
      installPlanApproval: Automatic
      name: ${OPERATOR}
      source: ${CATALOGSOURCE}
      sourceNamespace: openshift-marketplace
EOF
}

if [[ -n $(git status --porcelain) ]]; then
    echo "Uncommitted changes detected."
    exit 1
fi

echo "Checking for cluster reachability:"
OUT=$(oc cluster-info 2>&1)
ret=$?
if [ $ret -ne 0 ]; then
    echo "Could not reach cluster: ${OUT}"
    exit 1
fi

make VERSION=${VERSION} UPLOADREGISTRY="${UPLOADREGISTRY}" CHANNELS=fast USE_IMAGE_DIGESTS="" \
    manifests bundle generate docker-build docker-push console-build-amd64 console-push bundle-build bundle-push catalog-build \
    catalog-push catalog-install

# If the operator already exists in openshift-operators, keep using that namespace;
# otherwise use the new dedicated namespace.
if oc get subscriptions.operators.coreos.com "${OPERATOR}" -n openshift-operators &>/dev/null; then
    NS="openshift-operators"
    echo "Existing installation found in openshift-operators, upgrading in place"
else
    NS="${DEFAULT_NS}"
    echo "No existing installation found, installing in ${NS}"
fi

wait_for_resource "packagemanifest" "${OPERATOR}" "" "${CATALOGSOURCE}"

# Create namespace and OperatorGroup for dedicated namespace install
# (openshift-operators already has a global OperatorGroup, so skip for that namespace)
if [[ "${NS}" != "openshift-operators" ]]; then
    oc create namespace ${NS} --dry-run=client -o yaml | oc apply -f -
    oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: patterns-operator-group
  namespace: ${NS}
spec:
  targetNamespaces:
  - ${NS}
EOF
fi

apply_subscription
wait_for_resource "operator" "${OPERATOR}" "${NS}"

while true; do
    set +e
    INSTALLED_CSV=$(oc get subscriptions.operators.coreos.com "${OPERATOR}" -n "${NS}" -o jsonpath='{.status.installedCSV}')
    if [ -z "${INSTALLED_CSV}" ]; then
        sleep 10
    else
        break
    fi
done

wait_for_resource "csv" "${INSTALLED_CSV}" "${NS}"

