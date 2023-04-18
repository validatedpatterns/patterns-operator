# Patterns Operator - OpenShift Console Dynamic Plugin

At the moment this requires OpenShift 4.11. It may be possible to get this
working on OpenShift 4.10, if needed. For an example plugin that works with
OpenShift 4.10, see the `release-4.10` branch of
[openshift/console-plugin-template].

[OpenShift Console Dynamic plugins] allow you to extend the
[OpenShift Console] at runtime, adding custom pages and other extensions.
They are based on [Webpack Module Federation]. Plugins are registered with
console using the `ConsolePlugin` custom resource and enabled in the console
operator config by a cluster administrator.

## What This Plugin Does

This plugin provides a catalog of patterns in the OpenShift Console. This
catalog is similar in appearance and workflow to the OperatorHub catalog built
into OpenShift.

This plugin is installed automatically by the Patterns Operator. The plugin's
releases exist as a separate container image that is updated on each operator
release.

## Development

The following are required to build the plugin:

- [Node.js]
- [yarn]
- [podman] 3.2.0+ - **Preferred**
- [Docker] - Alternate

If you want to test against a real cluster, you will also need:

- A running [OpenShift Cluster]
- [OpenShift CLI] - oc


### Option 1: Local

In one terminal window, run:

1. `yarn install`
2. `yarn run start`

In another terminal window, run:

1. `oc login`
2. `yarn run start-console`

This will run the OpenShift console in a container connected to the cluster
you've logged into. The plugin HTTP server runs on port 9001 with CORS enabled.
Navigate to <http://localhost:9000/example> to see the running plugin.

#### Running start-console with Apple silicon and podman

If you are using podman on a Mac with Apple silicon, `yarn run start-console`
might fail since it runs an amd64 image. You can workaround the problem with
[qemu-user-static] by running these commands:

```bash
podman machine ssh
sudo -i
rpm-ostree install qemu-user-static
systemctl reboot
```

### Option 2: Docker + VSCode Remote Container

Make sure the [Remote Containers] extension is installed. This method uses
Docker Compose where one container is the OpenShift console and the second
container is the plugin. It requires that you have access to an existing
OpenShift cluster. After the initial build, the cached containers will help you
start developing in seconds.

1. Create a `dev.env` file inside the `.devcontainer` folder with the correct values for your cluster:

```bash
OC_PLUGIN_NAME=my-plugin
OC_URL=https://api.example.com:6443
OC_USER=kubeadmin
OC_PASS=<password>
```

2. `(Ctrl+Shift+P) => Remote Containers: Open Folder in Container...`
3. `yarn run start`
4. Navigate to <http://localhost:9000/example>

## Container image

Building the console plugin container image is done from the Makefile at the
root of the patterns-operator repo. It works the same way that `docker-build`
and `docker-push` do.

* `make console-build` - Build container image for console plugin
* `make console-push` - Push container image for console plugin

## Linting

This project adds prettier, eslint, and stylelint. Linting can be run with
`yarn run lint`.

The stylelint config disallows hex colors since these cause problems with dark
mode (starting in OpenShift console 4.11). You should use the
[PatternFly Global CSS Variables] for colors instead.

The stylelint config also disallows naked element selectors like `table` and
`.pf-` or `.co-` prefixed classes. This prevents plugins from accidentally
overwriting default console styles, breaking the layout of existing pages. The
best practice is to prefix your CSS classnames with your plugin name to avoid
conflicts. Please don't disable these rules without understanding how they can
break console styles!

## References

- [Console Plugin SDK README]
- [Customization Plugin Example]
- [Dynamic Plugin Enhancement Proposal]

[Console Plugin SDK README]: https://github.com/openshift/console/tree/master/frontend/packages/console-dynamic-plugin-sdk
[Customization Plugin Example]: https://github.com/spadgett/console-customization-plugin
[Docker]: https://www.docker.com
[Dynamic Plugin Enhancement Proposal]: https://github.com/openshift/enhancements/blob/master/enhancements/console/dynamic-plugins.md
[Node.js]: https://nodejs.org/en/
[OpenShift CLI]: https://console.redhat.com/openshift/downloads
[OpenShift Console Dynamic Plugins]: https://github.com/openshift/console/tree/master/frontend/packages/console-dynamic-plugin-sdk
[OpenShift Console]: https://github.com/openshift/console
[OpenShift cluster]: https://console.redhat.com/openshift/create
[PatternFly Global CSS Variables]: https://patternfly-react-main.surge.sh/developer-resources/global-css-variables#global-css-variables
[Podman]: https://podman.io
[Remote Containers]: https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers
[Webpack Module Federation]: https://webpack.js.org/concepts/module-federation
[Yarn]: https://yarnpkg.com
[openshift/console-plugin-template]: https://github.com/openshift/console-plugin-template
[qemu-user-static]: https://github.com/multiarch/qemu-user-static
