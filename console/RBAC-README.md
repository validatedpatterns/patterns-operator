# Console Plugin RBAC Configuration

This document explains the RBAC (Role-Based Access Control) setup for the patterns-operator console plugin cluster compatibility checking feature.

## Overview

The console plugin needs permission to query cluster nodes to perform compatibility checking for patterns. Instead of creating separate RBAC resources, the plugin uses the same service account and permissions as the patterns-operator.

## Architecture

### Shared Service Account Approach

The console plugin uses the same service account as the patterns-operator (`patterns-operator-controller-manager`) to inherit all necessary RBAC permissions. This eliminates the need for duplicate permission configurations.

### RBAC Annotations in Operator Code

The required permissions are defined as kubebuilder annotations in the operator's Go code:

```go
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list
//+kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=list;get
```

These annotations automatically generate the ClusterRole and ClusterRoleBinding when the operator is built and deployed.

### Required Permissions

The console plugin needs these permissions for compatibility checking:

- **Nodes**: Read access to cluster nodes for resource information
- **Machines**: Read access to machine API for cloud provider detection
- **Infrastructure**: Read access to cluster infrastructure configuration

## Implementation Details

### Operator Code Changes

The node access permissions are added to the operator controller file:

**`internal/controller/pattern_controller.go`**:
```go
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list
//+kubebuilder:rbac:groups=machine.openshift.io,resources=machines,verbs=get;list
```

### Console Plugin Deployment

The console plugin deployment is now managed through the operator's OLM bundle or directly via operator deployment manifests rather than a separate Helm chart. The plugin pods use the operator's service account to inherit RBAC permissions.

## Deployment

The console plugin is deployed through the patterns-operator itself. When the operator is installed, it automatically:

1. Creates the console plugin deployment using the operator's service account
2. Registers the console plugin with OpenShift Console
3. Enables the plugin for use

No separate deployment steps are needed for RBAC as the plugin inherits operator permissions.

## Troubleshooting

### Permission Denied Errors

If you see errors like:
- "Permission denied: Console plugin does not have permission to list cluster nodes"
- "Failed to load cluster node information - check console plugin RBAC permissions"

**Solutions:**
1. Verify the operator ClusterRole exists and includes node permissions:
   ```bash
   oc get clusterrole patterns-operator-manager-role -o yaml | grep -A5 -B5 nodes
   ```

2. Verify the console plugin uses the operator's service account:
   ```bash
   oc get deployment patterns-operator-console-plugin -o yaml | grep serviceAccountName
   ```

3. Check the operator service account has proper permissions:
   ```bash
   oc auth can-i list nodes --as=system:serviceaccount:patterns-operator-system:patterns-operator-controller-manager
   ```

4. Regenerate operator manifests if node permissions are missing:
   ```bash
   cd patterns-operator
   make manifests
   ```

### Debug Mode

For debugging, you can temporarily skip cluster compatibility checking:

```javascript
// In browser console:
localStorage.setItem('PATTERNS_DEBUG_SKIP_CLUSTER_CHECK', 'true');
```

This will disable compatibility checking until the flag is removed.

## Security Considerations

- **Minimal Permissions**: Only `get` and `list` permissions are granted, no modification rights
- **Cluster-Scoped**: Permissions are cluster-scoped as nodes are cluster-level resources
- **Read-Only**: All permissions are read-only for security
- **Optional Resources**: Machine API and infrastructure config permissions are optional enhancements

## Verification

To verify the setup is working:

1. **Check Plugin Logs**: Look for successful cluster info fetching in console plugin logs
2. **Test Compatibility UI**: Visit the pattern catalog and verify compatibility status is shown
3. **Browser Console**: Check for any permission-related errors in browser developer tools

## Manual Verification

You can manually test the permissions:

```bash
# Test as the service account (replace with actual names)
oc auth can-i list nodes --as=system:serviceaccount:<namespace>:<service-account-name>

# Should return "yes" if permissions are correct
```