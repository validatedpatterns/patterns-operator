#!/bin/bash
set -o pipefail

# Generates catalog/ directory by fetching pattern-metadata.yaml from all repos
# tagged with "pattern" in the validatedpatterns orgs.
#
# Output:
#   catalog/catalog.yaml          — lightweight index of pattern names
#   catalog/<name>/pattern.yaml   — full normalized metadata per pattern
#
# Dependencies: gh, yq (v4+), jq, base64

ORGS=("validatedpatterns" "validatedpatterns-sandbox")
TOPIC=${TOPIC:-pattern}
GENERATOR_VERSION="1.0"
CATALOG_DIR="catalog"

# Normalize a single pattern-metadata.yaml (JSON from yq) into catalog schema.
# Reads JSON on stdin, writes normalized JSON on stdout.
# Args: $1 = org name
normalize_pattern() {
    local org="$1"
    jq --arg org "$org" '
        # Split owners string into array
        .owners = (
            if (.owners | type) == "string" then
                .owners | split(",") | map(gsub("^\\s+|\\s+$"; ""))
            else
                .owners // []
            end
        ) |

        # Flatten platform wrapper in requirements
        # upstream: requirements.hub.compute.platform.aws -> we want: requirements.hub.compute.aws
        .requirements = (
            if .requirements then
                .requirements | to_entries | map(
                    .value = (
                        .value | to_entries | map(
                            if .value.platform then
                                .value = .value.platform
                            else
                                .
                            end
                        ) | from_entries
                    )
                ) | from_entries
            else
                null
            end
        ) |

        # Add org field
        .org = $org |

        # Ensure optional fields default to null
        .external_requirements = (.external_requirements // null) |
        .extra_features = (.extra_features // null) |
        .docs_repo_url = (.docs_repo_url // null) |
        .ci_url = (.ci_url // null) |
        .spoke = (.spoke // null)
    '
}

rm -rf "${CATALOG_DIR}"
mkdir -p "${CATALOG_DIR}"

pattern_names=()

for org in "${ORGS[@]}"; do
    repos=$(gh repo list "${org}" --no-archived --topic "${TOPIC}" --visibility public --limit 100 | awk '{ print $1 }' | sort)
    ret=$?
    if [ ${ret} -ne 0 ]; then
        echo "Error retrieving pattern list for ${org}" >&2
        exit ${ret}
    fi

    for full_slug in ${repos}; do
        repo_name="${full_slug#*/}"
        echo "Checking ${full_slug}..." >&2

        # Fetch pattern-metadata.yaml; skip if not found
        api_response=$(gh api "repos/${full_slug}/contents/pattern-metadata.yaml" 2>/dev/null)
        if [ $? -ne 0 ]; then
            echo "  No pattern-metadata.yaml, skipping." >&2
            continue
        fi

        # Decode and convert YAML to JSON
        raw_yaml=$(echo "${api_response}" | jq -r '.content' | base64 -d)
        if [ $? -ne 0 ]; then
            echo "  Failed to decode content, skipping." >&2
            continue
        fi

        pattern_json=$(echo "${raw_yaml}" | yq -o json '.' | normalize_pattern "${org}")
        if [ $? -ne 0 ]; then
            echo "  Failed to parse metadata, skipping." >&2
            continue
        fi

        # Write per-pattern YAML
        mkdir -p "${CATALOG_DIR}/${repo_name}"
        echo "${pattern_json}" | yq -P '.' > "${CATALOG_DIR}/${repo_name}/pattern.yaml"

        pattern_names+=("${repo_name}")
    done
done

# Build catalog index
{
    echo "generated_at: \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\""
    echo "generator_version: \"${GENERATOR_VERSION}\""
    echo "patterns:"
    for name in "${pattern_names[@]}"; do
        echo "  - ${name}"
    done
} > "${CATALOG_DIR}/catalog.yaml"

echo "Wrote ${CATALOG_DIR}/ with ${#pattern_names[@]} pattern(s)." >&2
