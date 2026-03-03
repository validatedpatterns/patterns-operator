export interface InstanceCapabilities {
  vcpus: number;
  memory: number; // GB
  category: 'small' | 'medium' | 'large' | 'xlarge';
}

/**
 * Instance type capabilities mapping for major cloud providers
 */
export const INSTANCE_CAPABILITIES: Record<string, Record<string, InstanceCapabilities>> = {
  aws: {
    // General Purpose M5 instances
    'm5.large': { vcpus: 2, memory: 8, category: 'small' },
    'm5.xlarge': { vcpus: 4, memory: 16, category: 'medium' },
    'm5.2xlarge': { vcpus: 8, memory: 32, category: 'large' },
    'm5.4xlarge': { vcpus: 16, memory: 64, category: 'xlarge' },
    'm5.8xlarge': { vcpus: 32, memory: 128, category: 'xlarge' },
    'm5.12xlarge': { vcpus: 48, memory: 192, category: 'xlarge' },
    'm5.16xlarge': { vcpus: 64, memory: 256, category: 'xlarge' },
    'm5.24xlarge': { vcpus: 96, memory: 384, category: 'xlarge' },
    'm5.metal': { vcpus: 96, memory: 384, category: 'xlarge' },

    // General Purpose M6i instances
    'm6i.large': { vcpus: 2, memory: 8, category: 'small' },
    'm6i.xlarge': { vcpus: 4, memory: 16, category: 'medium' },
    'm6i.2xlarge': { vcpus: 8, memory: 32, category: 'large' },
    'm6i.4xlarge': { vcpus: 16, memory: 64, category: 'xlarge' },
    'm6i.8xlarge': { vcpus: 32, memory: 128, category: 'xlarge' },
    'm6i.12xlarge': { vcpus: 48, memory: 192, category: 'xlarge' },
    'm6i.16xlarge': { vcpus: 64, memory: 256, category: 'xlarge' },
    'm6i.24xlarge': { vcpus: 96, memory: 384, category: 'xlarge' },
    'm6i.32xlarge': { vcpus: 128, memory: 512, category: 'xlarge' },

    // Compute Optimized C5 instances
    'c5.large': { vcpus: 2, memory: 4, category: 'small' },
    'c5.xlarge': { vcpus: 4, memory: 8, category: 'medium' },
    'c5.2xlarge': { vcpus: 8, memory: 16, category: 'large' },
    'c5.4xlarge': { vcpus: 16, memory: 32, category: 'xlarge' },
    'c5.9xlarge': { vcpus: 36, memory: 72, category: 'xlarge' },
    'c5.12xlarge': { vcpus: 48, memory: 96, category: 'xlarge' },
    'c5.18xlarge': { vcpus: 72, memory: 144, category: 'xlarge' },
    'c5.24xlarge': { vcpus: 96, memory: 192, category: 'xlarge' },

    // Memory Optimized R5 instances
    'r5.large': { vcpus: 2, memory: 16, category: 'small' },
    'r5.xlarge': { vcpus: 4, memory: 32, category: 'medium' },
    'r5.2xlarge': { vcpus: 8, memory: 64, category: 'large' },
    'r5.4xlarge': { vcpus: 16, memory: 128, category: 'xlarge' },
    'r5.8xlarge': { vcpus: 32, memory: 256, category: 'xlarge' },
    'r5.12xlarge': { vcpus: 48, memory: 384, category: 'xlarge' },
    'r5.16xlarge': { vcpus: 64, memory: 512, category: 'xlarge' },
    'r5.24xlarge': { vcpus: 96, memory: 768, category: 'xlarge' },

    // T3 burstable instances
    't3.medium': { vcpus: 2, memory: 4, category: 'small' },
    't3.large': { vcpus: 2, memory: 8, category: 'small' },
    't3.xlarge': { vcpus: 4, memory: 16, category: 'medium' },
    't3.2xlarge': { vcpus: 8, memory: 32, category: 'large' },
  },

  gcp: {
    // Standard machine types
    'n1-standard-1': { vcpus: 1, memory: 3.75, category: 'small' },
    'n1-standard-2': { vcpus: 2, memory: 7.5, category: 'small' },
    'n1-standard-4': { vcpus: 4, memory: 15, category: 'medium' },
    'n1-standard-8': { vcpus: 8, memory: 30, category: 'large' },
    'n1-standard-16': { vcpus: 16, memory: 60, category: 'xlarge' },
    'n1-standard-32': { vcpus: 32, memory: 120, category: 'xlarge' },
    'n1-standard-64': { vcpus: 64, memory: 240, category: 'xlarge' },
    'n1-standard-96': { vcpus: 96, memory: 360, category: 'xlarge' },

    // N2 Standard
    'n2-standard-2': { vcpus: 2, memory: 8, category: 'small' },
    'n2-standard-4': { vcpus: 4, memory: 16, category: 'medium' },
    'n2-standard-8': { vcpus: 8, memory: 32, category: 'large' },
    'n2-standard-16': { vcpus: 16, memory: 64, category: 'xlarge' },
    'n2-standard-32': { vcpus: 32, memory: 128, category: 'xlarge' },
    'n2-standard-48': { vcpus: 48, memory: 192, category: 'xlarge' },
    'n2-standard-64': { vcpus: 64, memory: 256, category: 'xlarge' },
    'n2-standard-80': { vcpus: 80, memory: 320, category: 'xlarge' },
    'n2-standard-96': { vcpus: 96, memory: 384, category: 'xlarge' },
    'n2-standard-128': { vcpus: 128, memory: 512, category: 'xlarge' },

    // High Memory
    'n1-highmem-2': { vcpus: 2, memory: 13, category: 'small' },
    'n1-highmem-4': { vcpus: 4, memory: 26, category: 'medium' },
    'n1-highmem-8': { vcpus: 8, memory: 52, category: 'large' },
    'n1-highmem-16': { vcpus: 16, memory: 104, category: 'xlarge' },
    'n1-highmem-32': { vcpus: 32, memory: 208, category: 'xlarge' },
    'n1-highmem-64': { vcpus: 64, memory: 416, category: 'xlarge' },
    'n1-highmem-96': { vcpus: 96, memory: 624, category: 'xlarge' },

    // High CPU
    'n1-highcpu-2': { vcpus: 2, memory: 1.8, category: 'small' },
    'n1-highcpu-4': { vcpus: 4, memory: 3.6, category: 'medium' },
    'n1-highcpu-8': { vcpus: 8, memory: 7.2, category: 'large' },
    'n1-highcpu-16': { vcpus: 16, memory: 14.4, category: 'xlarge' },
    'n1-highcpu-32': { vcpus: 32, memory: 28.8, category: 'xlarge' },
    'n1-highcpu-64': { vcpus: 64, memory: 57.6, category: 'xlarge' },
    'n1-highcpu-96': { vcpus: 96, memory: 86.4, category: 'xlarge' },
  },

  azure: {
    // Standard D-series v3
    'Standard_D2s_v3': { vcpus: 2, memory: 8, category: 'small' },
    'Standard_D4s_v3': { vcpus: 4, memory: 16, category: 'medium' },
    'Standard_D8s_v3': { vcpus: 8, memory: 32, category: 'large' },
    'Standard_D16s_v3': { vcpus: 16, memory: 64, category: 'xlarge' },
    'Standard_D32s_v3': { vcpus: 32, memory: 128, category: 'xlarge' },
    'Standard_D48s_v3': { vcpus: 48, memory: 192, category: 'xlarge' },
    'Standard_D64s_v3': { vcpus: 64, memory: 256, category: 'xlarge' },

    // Standard D-series v4
    'Standard_D2s_v4': { vcpus: 2, memory: 8, category: 'small' },
    'Standard_D4s_v4': { vcpus: 4, memory: 16, category: 'medium' },
    'Standard_D8s_v4': { vcpus: 8, memory: 32, category: 'large' },
    'Standard_D16s_v4': { vcpus: 16, memory: 64, category: 'xlarge' },
    'Standard_D32s_v4': { vcpus: 32, memory: 128, category: 'xlarge' },
    'Standard_D48s_v4': { vcpus: 48, memory: 192, category: 'xlarge' },
    'Standard_D64s_v4': { vcpus: 64, memory: 256, category: 'xlarge' },

    // Standard E-series v3 (memory optimized)
    'Standard_E2s_v3': { vcpus: 2, memory: 16, category: 'small' },
    'Standard_E4s_v3': { vcpus: 4, memory: 32, category: 'medium' },
    'Standard_E8s_v3': { vcpus: 8, memory: 64, category: 'large' },
    'Standard_E16s_v3': { vcpus: 16, memory: 128, category: 'xlarge' },
    'Standard_E32s_v3': { vcpus: 32, memory: 256, category: 'xlarge' },
    'Standard_E48s_v3': { vcpus: 48, memory: 384, category: 'xlarge' },
    'Standard_E64s_v3': { vcpus: 64, memory: 432, category: 'xlarge' },

    // Standard F-series v2 (compute optimized)
    'Standard_F2s_v2': { vcpus: 2, memory: 4, category: 'small' },
    'Standard_F4s_v2': { vcpus: 4, memory: 8, category: 'medium' },
    'Standard_F8s_v2': { vcpus: 8, memory: 16, category: 'large' },
    'Standard_F16s_v2': { vcpus: 16, memory: 32, category: 'xlarge' },
    'Standard_F32s_v2': { vcpus: 32, memory: 64, category: 'xlarge' },
    'Standard_F48s_v2': { vcpus: 48, memory: 96, category: 'xlarge' },
    'Standard_F64s_v2': { vcpus: 64, memory: 128, category: 'xlarge' },
    'Standard_F72s_v2': { vcpus: 72, memory: 144, category: 'xlarge' },

    // Standard B-series (burstable)
    'Standard_B1ms': { vcpus: 1, memory: 2, category: 'small' },
    'Standard_B1s': { vcpus: 1, memory: 1, category: 'small' },
    'Standard_B2ms': { vcpus: 2, memory: 8, category: 'small' },
    'Standard_B2s': { vcpus: 2, memory: 4, category: 'small' },
    'Standard_B4ms': { vcpus: 4, memory: 16, category: 'medium' },
    'Standard_B8ms': { vcpus: 8, memory: 32, category: 'large' },
    'Standard_B12ms': { vcpus: 12, memory: 48, category: 'xlarge' },
    'Standard_B16ms': { vcpus: 16, memory: 64, category: 'xlarge' },
    'Standard_B20ms': { vcpus: 20, memory: 80, category: 'xlarge' },
  }
};

/**
 * Gets the capabilities for a specific instance type
 * @param cloudProvider The cloud provider (aws, gcp, azure)
 * @param instanceType The instance type name
 * @returns InstanceCapabilities object or null if not found
 */
export function getInstanceCapabilities(
  cloudProvider: string,
  instanceType: string
): InstanceCapabilities | null {
  const providerInstances = INSTANCE_CAPABILITIES[cloudProvider];
  if (!providerInstances) {
    return null;
  }

  return providerInstances[instanceType] || null;
}

/**
 * Compares if an available instance type meets or exceeds the requirements
 * @param requiredInstanceType The required instance type from pattern
 * @param availableInstanceType The available instance type on cluster
 * @param cloudProvider The cloud provider
 * @returns true if available instance meets or exceeds requirements
 */
export function compareInstanceCapabilities(
  requiredInstanceType: string,
  availableInstanceType: string | undefined,
  cloudProvider: string
): boolean {
  if (!availableInstanceType) {
    return false;
  }

  const requiredCapabilities = getInstanceCapabilities(cloudProvider, requiredInstanceType);
  const availableCapabilities = getInstanceCapabilities(cloudProvider, availableInstanceType);

  if (!requiredCapabilities || !availableCapabilities) {
    // If we can't determine capabilities, be conservative but allow
    return false;
  }

  // Check if available instance meets or exceeds requirements
  return (
    availableCapabilities.vcpus >= requiredCapabilities.vcpus &&
    availableCapabilities.memory >= requiredCapabilities.memory
  );
}

/**
 * Estimates instance category based on vCPUs and memory if exact type is unknown
 * @param vcpus Number of vCPUs
 * @param memory Memory in GB
 * @returns Estimated instance category
 */
export function estimateInstanceCategory(vcpus: number, memory: number): InstanceCapabilities['category'] {
  if (vcpus >= 16 || memory >= 64) {
    return 'xlarge';
  } else if (vcpus >= 8 || memory >= 32) {
    return 'large';
  } else if (vcpus >= 4 || memory >= 16) {
    return 'medium';
  } else {
    return 'small';
  }
}

/**
 * Gets the human-readable description for an instance type
 * @param cloudProvider The cloud provider
 * @param instanceType The instance type
 * @returns Human readable description or the instance type itself
 */
export function getInstanceTypeDescription(cloudProvider: string, instanceType: string): string {
  const capabilities = getInstanceCapabilities(cloudProvider, instanceType);
  if (!capabilities) {
    return instanceType;
  }

  return `${instanceType} (${capabilities.vcpus} vCPUs, ${capabilities.memory}GB RAM)`;
}

/**
 * Finds the best matching instance types for given requirements
 * @param cloudProvider The cloud provider
 * @param minVcpus Minimum vCPUs required
 * @param minMemory Minimum memory in GB required
 * @returns Array of suitable instance types sorted by cost-effectiveness
 */
export function findSuitableInstanceTypes(
  cloudProvider: string,
  minVcpus: number,
  minMemory: number
): string[] {
  const providerInstances = INSTANCE_CAPABILITIES[cloudProvider];
  if (!providerInstances) {
    return [];
  }

  const suitable = Object.entries(providerInstances)
    .filter(([, capabilities]) =>
      capabilities.vcpus >= minVcpus && capabilities.memory >= minMemory
    )
    .sort(([, a], [, b]) => {
      // Sort by total compute units (vcpus + memory/8) for rough cost estimation
      const scoreA = a.vcpus + a.memory / 8;
      const scoreB = b.vcpus + b.memory / 8;
      return scoreA - scoreB;
    })
    .map(([instanceType]) => instanceType);

  return suitable;
}