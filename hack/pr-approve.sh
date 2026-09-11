#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<EOF
Usage: $(basename "$0") <pr-number-or-url>

Post three separate comments on a GitHub PR:
  1) /ok-to-test
  2) /lgtm
  3) /approve
EOF
}

if [[ $# -lt 1 ]]; then
  usage
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh CLI is required but was not found in PATH" >&2
  exit 1
fi

PR="$1"

for comment in "/ok-to-test" "/lgtm" "/approve"; do
  echo "Posting comment: ${comment}"
  gh pr comment "${PR}" --body "${comment}"
done

echo "Done. Posted /ok-to-test, /lgtm, and /approve on ${PR}."
