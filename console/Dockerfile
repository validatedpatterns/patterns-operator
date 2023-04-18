# TODO: console-plugin-template used to use this image as it's parent. It has
# since switched to a UBI 8 image. Our image should be updated. It's going to
# be more involved than swapping them out. I swapped them out and got errors
# when setting permissions.
FROM quay.io/cajieh0/nginx:stable

COPY ./dist /usr/share/nginx/html

# This image will not run without a TLS certificate and key injected into
# /var/cert. The OpenShift service and deployment will handle this
# automatically. If for some reason you need to test this outside of OpenShift,
# either mount a certificate/key pair or remove this section.
COPY ./hack/nginx.conf /etc/nginx/nginx.conf

RUN chmod g+rwx /etc/nginx/nginx.conf /var/cache/nginx /var/log/nginx /var/run
