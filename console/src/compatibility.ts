import { Pattern } from './types';
import { ClusterInfo } from './cluster-api';
import { compareInstanceCapabilities } from './utils/instance-types';

export type CompatibilityStatus = 'compatible' | 'insufficient' | 'unknown' | 'error';

export interface CompatibilityResult {
  status: CompatibilityStatus;
  reason: string;
  details?: {
    requiredCompute?: { replicas: number; type: string };
    requiredControlPlane?: { replicas: number; type: string };
    actualCompute?: { count: number; types: string[] };
    actualControlPlane?: { count: number; types: string[] };
    cloudProviderMatch?: boolean;
  };
}

/**
 * Checks if a pattern can be installed on the current cluster
 * @param pattern The pattern to check compatibility for
 * @param clusterInfo Current cluster information
 * @returns CompatibilityResult indicating if pattern can be installed
 */
export function checkPatternCompatibility(
  pattern: Pattern,
  clusterInfo: ClusterInfo,
): CompatibilityResult {
  try {
    // 1. Handle baremetal clusters
    if (clusterInfo.isBaremetal) {
      return {
        status: 'unknown',
        reason: 'Cannot determine compatibility for baremetal clusters. Manual verification required.',
        details: {
          actualCompute: {
            count: clusterInfo.totalWorkerNodes,
            types: clusterInfo.workerNodes.map(n => n.instanceType || 'unknown'),
          },
          actualControlPlane: {
            count: clusterInfo.totalControlPlaneNodes,
            types: clusterInfo.controlPlaneNodes.map(n => n.instanceType || 'unknown'),
          },
          cloudProviderMatch: false,
        },
      };
    }

    // 2. Check if pattern has requirements
    const hubRequirements = pattern.requirements?.hub;
    if (!hubRequirements) {
      return {
        status: 'unknown',
        reason: 'Pattern does not specify resource requirements. Compatibility cannot be determined.',
      };
    }

    // 3. Check if cluster cloud provider is unknown
    if (!clusterInfo.cloudProvider || clusterInfo.cloudProvider === 'unknown') {
      return {
        status: 'unknown',
        reason: 'Could not determine cluster cloud provider. Manual verification required.',
      };
    }

    // 4. Check cloud provider support
    const computeRequirements = hubRequirements.compute;
    const controlPlaneRequirements = hubRequirements.controlPlane;

    const supportedProviders = [
      ...(computeRequirements ? Object.keys(computeRequirements) : []),
      ...(controlPlaneRequirements ? Object.keys(controlPlaneRequirements) : []),
    ];

    if (!supportedProviders.includes(clusterInfo.cloudProvider)) {
      return {
        status: 'insufficient',
        reason: `Pattern does not support ${clusterInfo.cloudProvider} cloud provider. Supported providers: ${supportedProviders.join(', ')}.`,
        details: {
          cloudProviderMatch: false,
          actualCompute: {
            count: clusterInfo.totalWorkerNodes,
            types: clusterInfo.workerNodes.map(n => n.instanceType || 'unknown'),
          },
          actualControlPlane: {
            count: clusterInfo.totalControlPlaneNodes,
            types: clusterInfo.controlPlaneNodes.map(n => n.instanceType || 'unknown'),
          },
        },
      };
    }

    // 5. Check compute node requirements
    if (computeRequirements && computeRequirements[clusterInfo.cloudProvider]) {
      const computeSpec = computeRequirements[clusterInfo.cloudProvider];

      // Check replica count
      if (computeSpec.replicas > clusterInfo.totalWorkerNodes) {
        return {
          status: 'insufficient',
          reason: `Insufficient worker nodes: ${computeSpec.replicas} required, ${clusterInfo.totalWorkerNodes} available.`,
          details: {
            requiredCompute: { replicas: computeSpec.replicas, type: computeSpec.type },
            actualCompute: {
              count: clusterInfo.totalWorkerNodes,
              types: clusterInfo.workerNodes.map(n => n.instanceType || 'unknown'),
            },
            cloudProviderMatch: true,
          },
        };
      }

      // Check instance type capabilities
      const hasCapableNodes = clusterInfo.workerNodes.some(node =>
        compareInstanceCapabilities(computeSpec.type, node.instanceType, clusterInfo.cloudProvider!)
      );

      if (!hasCapableNodes) {
        return {
          status: 'insufficient',
          reason: `No worker nodes meet minimum compute requirements. Required: ${computeSpec.type} or equivalent.`,
          details: {
            requiredCompute: { replicas: computeSpec.replicas, type: computeSpec.type },
            actualCompute: {
              count: clusterInfo.totalWorkerNodes,
              types: clusterInfo.workerNodes.map(n => n.instanceType || 'unknown'),
            },
            cloudProviderMatch: true,
          },
        };
      }

      // Check if we have enough capable nodes (not just any capable nodes)
      const capableNodeCount = clusterInfo.workerNodes.filter(node =>
        compareInstanceCapabilities(computeSpec.type, node.instanceType, clusterInfo.cloudProvider!)
      ).length;

      if (capableNodeCount < computeSpec.replicas) {
        return {
          status: 'insufficient',
          reason: `Insufficient capable worker nodes: ${computeSpec.replicas} required of type ${computeSpec.type} or better, ${capableNodeCount} available.`,
          details: {
            requiredCompute: { replicas: computeSpec.replicas, type: computeSpec.type },
            actualCompute: {
              count: clusterInfo.totalWorkerNodes,
              types: clusterInfo.workerNodes.map(n => n.instanceType || 'unknown'),
            },
            cloudProviderMatch: true,
          },
        };
      }
    }

    // 6. Check control plane requirements
    if (controlPlaneRequirements && controlPlaneRequirements[clusterInfo.cloudProvider]) {
      const controlPlaneSpec = controlPlaneRequirements[clusterInfo.cloudProvider];

      // Check replica count
      if (controlPlaneSpec.replicas > clusterInfo.totalControlPlaneNodes) {
        return {
          status: 'insufficient',
          reason: `Insufficient control plane nodes: ${controlPlaneSpec.replicas} required, ${clusterInfo.totalControlPlaneNodes} available.`,
          details: {
            requiredControlPlane: { replicas: controlPlaneSpec.replicas, type: controlPlaneSpec.type },
            actualControlPlane: {
              count: clusterInfo.totalControlPlaneNodes,
              types: clusterInfo.controlPlaneNodes.map(n => n.instanceType || 'unknown'),
            },
            cloudProviderMatch: true,
          },
        };
      }

      // Check instance type capabilities for control plane
      const hasCapableControlPlaneNodes = clusterInfo.controlPlaneNodes.some(node =>
        compareInstanceCapabilities(controlPlaneSpec.type, node.instanceType, clusterInfo.cloudProvider!)
      );

      if (!hasCapableControlPlaneNodes) {
        return {
          status: 'insufficient',
          reason: `No control plane nodes meet minimum requirements. Required: ${controlPlaneSpec.type} or equivalent.`,
          details: {
            requiredControlPlane: { replicas: controlPlaneSpec.replicas, type: controlPlaneSpec.type },
            actualControlPlane: {
              count: clusterInfo.totalControlPlaneNodes,
              types: clusterInfo.controlPlaneNodes.map(n => n.instanceType || 'unknown'),
            },
            cloudProviderMatch: true,
          },
        };
      }

      // Check if we have enough capable control plane nodes
      const capableControlPlaneCount = clusterInfo.controlPlaneNodes.filter(node =>
        compareInstanceCapabilities(controlPlaneSpec.type, node.instanceType, clusterInfo.cloudProvider!)
      ).length;

      if (capableControlPlaneCount < controlPlaneSpec.replicas) {
        return {
          status: 'insufficient',
          reason: `Insufficient capable control plane nodes: ${controlPlaneSpec.replicas} required of type ${controlPlaneSpec.type} or better, ${capableControlPlaneCount} available.`,
          details: {
            requiredControlPlane: { replicas: controlPlaneSpec.replicas, type: controlPlaneSpec.type },
            actualControlPlane: {
              count: clusterInfo.totalControlPlaneNodes,
              types: clusterInfo.controlPlaneNodes.map(n => n.instanceType || 'unknown'),
            },
            cloudProviderMatch: true,
          },
        };
      }
    }

    // 7. All checks passed - pattern is compatible
    return {
      status: 'compatible',
      reason: 'Cluster meets all pattern requirements.',
      details: {
        requiredCompute: computeRequirements?.[clusterInfo.cloudProvider]
          ? {
              replicas: computeRequirements[clusterInfo.cloudProvider].replicas,
              type: computeRequirements[clusterInfo.cloudProvider].type
            }
          : undefined,
        requiredControlPlane: controlPlaneRequirements?.[clusterInfo.cloudProvider]
          ? {
              replicas: controlPlaneRequirements[clusterInfo.cloudProvider].replicas,
              type: controlPlaneRequirements[clusterInfo.cloudProvider].type
            }
          : undefined,
        actualCompute: {
          count: clusterInfo.totalWorkerNodes,
          types: clusterInfo.workerNodes.map(n => n.instanceType || 'unknown'),
        },
        actualControlPlane: {
          count: clusterInfo.totalControlPlaneNodes,
          types: clusterInfo.controlPlaneNodes.map(n => n.instanceType || 'unknown'),
        },
        cloudProviderMatch: true,
      },
    };
  } catch (error) {
    console.error('Error checking pattern compatibility:', error);
    return {
      status: 'error',
      reason: `Failed to check compatibility: ${error instanceof Error ? error.message : 'Unknown error'}`,
    };
  }
}

/**
 * Gets a user-friendly color for the compatibility status
 * @param status The compatibility status
 * @returns PatternFly color scheme name
 */
export function getCompatibilityColor(status: CompatibilityStatus): 'green' | 'red' | 'orange' | 'grey' {
  switch (status) {
    case 'compatible':
      return 'green';
    case 'insufficient':
      return 'red';
    case 'unknown':
      return 'orange';
    case 'error':
      return 'grey';
    default:
      return 'grey';
  }
}

/**
 * Gets a user-friendly label for the compatibility status
 * @param status The compatibility status
 * @returns Human readable status label
 */
export function getCompatibilityLabel(status: CompatibilityStatus): string {
  switch (status) {
    case 'compatible':
      return 'Compatible';
    case 'insufficient':
      return 'Insufficient Resources';
    case 'unknown':
      return 'Cannot Determine';
    case 'error':
      return 'Check Failed';
    default:
      return 'Unknown';
  }
}

/**
 * Gets appropriate install button text based on compatibility status
 * @param status The compatibility status
 * @returns Button text for install action
 */
export function getInstallButtonText(status: CompatibilityStatus): string {
  switch (status) {
    case 'compatible':
      return 'Install';
    case 'insufficient':
      return 'Install Anyway';
    case 'unknown':
      return 'Install';
    case 'error':
      return 'Install';
    default:
      return 'Install';
  }
}

/**
 * Determines if the install button should be disabled based on compatibility
 * @param status The compatibility status
 * @returns true if button should be disabled
 */
export function shouldDisableInstall(status: CompatibilityStatus): boolean {
  // We don't disable install for any status - let users proceed with warnings
  // This allows for edge cases and manual overrides
  return false;
}

/**
 * Gets the button variant based on compatibility status
 * @param status The compatibility status
 * @returns PatternFly button variant
 */
export function getInstallButtonVariant(status: CompatibilityStatus): 'primary' | 'secondary' | 'danger' {
  switch (status) {
    case 'compatible':
      return 'primary';
    case 'insufficient':
      return 'secondary';
    case 'unknown':
      return 'secondary';
    case 'error':
      return 'secondary';
    default:
      return 'secondary';
  }
}