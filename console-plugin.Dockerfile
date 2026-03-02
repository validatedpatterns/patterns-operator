FROM registry.access.redhat.com/ubi9/nodejs-20:latest AS build
USER root
RUN command -v yarn || npm i -g yarn

WORKDIR /usr/src/app
COPY console/ .
# replace version in package.json
RUN sed -r -i "s|\"version\": \"0.0.1\"|\"version\": \"6.6.6\"|;" ./package.json
RUN yarn install && yarn build

FROM registry.access.redhat.com/ubi9/nginx-120:latest
LABEL \
    com.redhat.openshift.versions="v4.20-v4.21" \
    com.redhat.component="Console plugin image for OpenShift Pattern Operator" \
    description="This is the console plugin for the OpenShift Pattern Operator" \
    io.k8s.display-name="Console plugin image for OpenShift Pattern Operator" \
    io.k8s.description="" \
    io.openshift.tags="openshift,patterns" \
    distribution-scope="public" \
    name="patterns-operator-console-plugin" \
    summary="Pattern Console Plugin" \
    release="v6.6.6" \
    version="v6.6.6" \
    maintainer="https://groups.google.com/g/validatedpatterns" \
    url="https://github.com/validatedpatterns/patterns-operator.git" \
    vendor="Red Hat" \
    License="Apache License 2.0"

COPY --from=build /usr/src/app/dist /usr/share/nginx/html
RUN mkdir licenses
COPY LICENSE licenses/
USER 1001

ENTRYPOINT ["nginx", "-g", "daemon off;"]
