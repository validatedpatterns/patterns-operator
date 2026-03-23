#!/usr/bin/env bash

set -euo pipefail
CONSOLE_IMAGE_PLATFORM=${CONSOLE_IMAGE_PLATFORM:="linux/amd64"}


echo "Starting local UI catalog image..."
echo "Console Image: $UI_CATALOG_IMG"


# Prefer podman if installed. Otherwise, fall back to docker.
if [ -x "$(command -v podman)" ]; then
    podman run --pull always --platform $CONSOLE_IMAGE_PLATFORM --rm -p 8080:8080 \
    --user 0 --entrypoint sh $UI_CATALOG_IMG -c "sed -i 's|root .*;|root /usr/share/nginx/html;|g' /etc/nginx/nginx.conf && nginx -g 'daemon off;'"

else
    BRIDGE_PLUGINS="${PLUGIN_NAME}=http://host.docker.internal:9001"
    docker run --pull always --platform $CONSOLE_IMAGE_PLATFORM --rm -p 8080:8080 \
    --user 0 --entrypoint sh $UI_CATALOG_IMG -c "sed -i 's|root .*;|root /usr/share/nginx/html;|g' /etc/nginx/nginx.conf && nginx -g 'daemon off;'"

fi
