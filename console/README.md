# OpenShift Console Plugin Template

This project is a minimal template for writing a new OpenShift Console dynamic
plugin.

[Dynamic plugins](https://github.com/openshift/console/tree/master/frontend/packages/console-dynamic-plugin-sdk)
allow you to extend the
[OpenShift UI](https://github.com/openshift/console)
at runtime, adding custom pages and other extensions. They are based on
[webpack module federation](https://webpack.js.org/concepts/module-federation/).
Plugins are registered with console using the `ConsolePlugin` custom resource
and enabled in the console operator config by a cluster administrator.

Using the latest `v1` API version of `ConsolePlugin` CRD, requires OpenShift 4.12
and higher. For using old `v1alpha1` API version us OpenShift version 4.10 or 4.11.

For an example of a plugin that works with OpenShift 4.11, see the `release-4.11` branch.
For a plugin that works with OpenShift 4.10, see the `release-4.10` branch.

[Node.js](https://nodejs.org/en/) and [yarn](https://yarnpkg.com) are required
to build and run the example. To run OpenShift console in a container, either
[Docker](https://www.docker.com) or [podman 3.2.0+](https://podman.io) and
[oc](https://console.redhat.com/openshift/downloads) are required.

## Getting started

> [!IMPORTANT]  
> To use this template, **DO NOT FORK THIS REPOSITORY**! Click **Use this template**, then select
> [**Create a new repository**](https://github.com/new?template_name=networking-console-plugin&template_owner=openshift)
> to create a new repository.
>
> ![A screenshot showing where the "Use this template" button is located](https://i.imgur.com/AhaySbU.png)
>
> **Forking this repository** for purposes outside of contributing to this repository
> **will cause issues**, as users cannot have more than one fork of a template repository
> at a time. This could prevent future users from forking and contributing to your plugin.
> 
> Your fork would also behave like a template repository, which might be confusing for
> contributiors, because it is not possible for repositories generated from a template
> repository to contribute back to the template.

After cloning your instantiated repository, you must update the plugin metadata, such as the
plugin name in the `consolePlugin` declaration of [package.json](package.json).

```json
"consolePlugin": {
  "name": "console-plugin-template",
  "version": "0.0.1",
  "displayName": "My Plugin",
  "description": "Enjoy this shiny, new console plugin!",
  "exposedModules": {
    "PatternCatalogPage": "./components/PatternCatalogPage"
  },
  "dependencies": {
    "@console/pluginAPI": "*"
  }
}
```

The template adds a single example page in the Home navigation section. The
extension is declared in the [console-extensions.json](console-extensions.json)
file and the React component is declared in
[src/components/PatternCatalogPage.tsx](src/components/PatternCatalogPage.tsx).

You can run the plugin using a local development environment or build an image
to deploy it to a cluster.

## Development

### Option 1: Local

In one terminal window, run:

1. `yarn install`
2. `yarn run start`

In another terminal window, run:

1. `oc login` (requires [oc](https://console.redhat.com/openshift/downloads) and an [OpenShift cluster](https://console.redhat.com/openshift/create))
2. `yarn run start-console` (requires [Docker](https://www.docker.com) or [podman 3.2.0+](https://podman.io))

This will run the OpenShift console in a container connected to the cluster
you've logged into. The plugin HTTP server runs on port 9001 with CORS enabled.
Navigate to <http://localhost:9000/example> to see the running plugin.

#### Running start-console with Apple silicon and podman

If you are using podman on a Mac with Apple silicon, `yarn run start-console`
might fail since it runs an amd64 image. You can workaround the problem with
[qemu-user-static](https://github.com/multiarch/qemu-user-static) by running
these commands:

```bash
podman machine ssh
sudo -i
rpm-ostree install qemu-user-static
systemctl reboot
```

### Option 2: Docker + VSCode Remote Container

Make sure the
[Remote Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
extension is installed. This method uses Docker Compose where one container is
the OpenShift console and the second container is the plugin. It requires that
you have access to an existing OpenShift cluster. After the initial build, the
cached containers will help you start developing in seconds.

1. Create a `dev.env` file inside the `.devcontainer` folder with the correct values for your cluster:

```bash
OC_PLUGIN_NAME=console-plugin-template
OC_URL=https://api.example.com:6443
OC_USER=kubeadmin
OC_PASS=<password>
```

2. `(Ctrl+Shift+P) => Remote Containers: Open Folder in Container...`
3. `yarn run start`
4. Navigate to <http://localhost:9000/example>

## Docker image

Before you can deploy your plugin on a cluster, you must build an image and
push it to an image registry.

1. Build the image:

   ```sh
   docker build -t quay.io/my-repository/my-plugin:latest .
   ```

2. Run the image:

   ```sh
   docker run -it --rm -d -p 9001:80 quay.io/my-repository/my-plugin:latest
   ```

3. Push the image:

   ```sh
   docker push quay.io/my-repository/my-plugin:latest
   ```

NOTE: If you have a Mac with Apple silicon, you will need to add the flag
`--platform=linux/amd64` when building the image to target the correct platform
to run in-cluster.

## Deployment on cluster

This console plugin is deployed automatically through the patterns-operator. There is no separate deployment step required.

### Operator-Managed Deployment

When the patterns-operator is installed via OLM (Operator Lifecycle Manager), it:

1. **Creates the console plugin deployment** using the operator's container image
2. **Registers the plugin** with OpenShift Console via ConsolePlugin resource
3. **Enables the plugin** automatically in the console configuration
4. **Provides RBAC permissions** for cluster compatibility checking through the operator's service account

### Manual Console Plugin Management

If you need to manually manage the console plugin state:

```bash
# Check if plugin is registered
oc get consoleplugin patterns-operator-console-plugin

# Check if plugin is enabled in console
oc get console cluster -o yaml | grep patterns-operator-console-plugin

# Manually enable plugin (usually automatic)
oc patch console cluster --type='merge' --patch='{"spec":{"plugins":["patterns-operator-console-plugin"]}}'
```

### Development Deployment

For development purposes, you can build and run the plugin separately:

```bash
# Build development image
docker build -t my-console-plugin:latest .

# Run locally (requires cluster access)
yarn start
```

NOTE: When defining i18n namespace, adhere `plugin__<name-of-the-plugin>` format. The name of the plugin should be extracted from the `consolePlugin` declaration within the [package.json](package.json) file.

## i18n

The plugin template demonstrates how you can translate messages in with [react-i18next](https://react.i18next.com/). The i18n namespace must match
the name of the `ConsolePlugin` resource with the `plugin__` prefix to avoid
naming conflicts. For example, the plugin template uses the
`plugin__console-plugin-template` namespace. You can use the `useTranslation` hook
with this namespace as follows:

```tsx
conster Header: React.FC = () => {
  const { t } = useTranslation('plugin__console-plugin-template');
  return <h1>{t('Hello, World!')}</h1>;
};
```

For labels in `console-extensions.json`, you can use the format
`%plugin__console-plugin-template~My Label%`. Console will replace the value with
the message for the current language from the `plugin__console-plugin-template`
namespace. For example:

```json
  {
    "type": "console.navigation/section",
    "properties": {
      "id": "admin-demo-section",
      "perspective": "admin",
      "name": "%plugin__console-plugin-template~Plugin Template%"
    }
  }
```

Running `yarn i18n` updates the JSON files in the `locales` folder of the
plugin template when adding or changing messages.

## Linting

This project adds prettier, eslint, and stylelint. Linting can be run with
`yarn run lint`.

The stylelint config disallows hex colors since these cause problems with dark
mode (starting in OpenShift console 4.11). You should use the
[PatternFly global CSS variables](https://patternfly-react-main.surge.sh/developer-resources/global-css-variables#global-css-variables)
for colors instead.

The stylelint config also disallows naked element selectors like `table` and
`.pf-` or `.co-` prefixed classes. This prevents plugins from accidentally
overwriting default console styles, breaking the layout of existing pages. The
best practice is to prefix your CSS classnames with your plugin name to avoid
conflicts. Please don't disable these rules without understanding how they can
break console styles!

## Reporting

Steps to generate reports

1. In command prompt, navigate to root folder and execute the command `yarn run cypress-merge`
2. Then execute command `yarn run cypress-generate`
The cypress-report.html file is generated and should be in (/integration-tests/screenshots) directory

## RBAC Requirements

This console plugin includes cluster compatibility checking functionality that requires access to cluster nodes. Instead of creating separate RBAC resources, the plugin uses the same service account as the patterns-operator to inherit the necessary permissions.

### Architecture

- **Service Account**: Console plugin uses `patterns-operator-controller-manager`
- **Permissions**: Inherited from operator's RBAC annotations in Go code
- **No separate RBAC**: All permissions managed through operator annotations

### Required Permissions

The following permissions are defined in the operator's Go controller:

```go
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=list;get
```

### Troubleshooting RBAC Issues

If you see permission-related errors:

1. **Verify operator permissions include nodes:**
   ```bash
   oc get clusterrole patterns-operator-manager-role -o yaml | grep -A3 -B3 nodes
   ```

2. **Check console plugin uses operator service account:**
   ```bash
   oc get deployment patterns-operator-console-plugin -o yaml | grep serviceAccountName
   ```

3. **Test permissions manually:**
   ```bash
   oc auth can-i list nodes --as=system:serviceaccount:patterns-operator-system:patterns-operator-controller-manager
   ```

4. **Regenerate manifests if needed:**
   ```bash
   make manifests  # In the operator root directory
   ```

For detailed RBAC configuration information, see [RBAC-README.md](./RBAC-README.md).

## References

- [Console Plugin SDK README](https://github.com/openshift/console/tree/master/frontend/packages/console-dynamic-plugin-sdk)
- [Customization Plugin Example](https://github.com/spadgett/console-customization-plugin)
- [Dynamic Plugin Enhancement Proposal](https://github.com/openshift/enhancements/blob/master/enhancements/console/dynamic-plugins.md)
