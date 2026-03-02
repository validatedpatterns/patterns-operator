#!/bin/bash
set -o pipefail

# Lists all repos tagged with "pattern" in the two organizations:
# - validatedpatterns
# - validatedpatterns-sandbox

ORGS=("validatedpatterns" "validatedpatterns-sandbox")
TOPIC=${TOPIC:-pattern}

for org in ${ORGS[@]}; do
    REPOS=$(gh repo list "${org}" --no-archived --topic "${TOPIC}" --visibility public --limit 100 |awk '{ print $1 }' | sort)
    ret=$?
    if [ ${ret} -ne 0 ]; then
        echo "Error retrieving pattern list for ${org}"
        exit ${ret}
    fi
    echo "${REPOS}"
done
