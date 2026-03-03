import { k8sList } from '@openshift-console/dynamic-plugin-sdk';
import { K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';
import { useState, useEffect } from 'react';

export interface ClusterNode extends K8sResourceCommon {
  metadata: {
    name: string;
    labels?: {
      [key: string]: string;
    };
    annotations?: {
      [key: string]: string;
    };
  };
  spec?: {
    providerID?: string;
  };
  status?: {
    conditions?: Array<{
      type: string;
      status: string;
    }>;
    nodeInfo?: {
      architecture?: string;
      operatingSystem?: string;
    };
  };
}

export interface ParsedClusterNode {
  name: string;
  instanceType?: string;
  cloudProvider?: string;
  roles: string[]; // 'master', 'worker'
  ready: boolean;
  vcpus?: number;
  memory?: number;
}

export interface ClusterInfo {
  nodes: ParsedClusterNode[];
  cloudProvider?: 'aws' | 'gcp' | 'azure' | 'baremetal' | 'unknown';
  workerNodes: ParsedClusterNode[];
  controlPlaneNodes: ParsedClusterNode[];
  isBaremetal: boolean;
  totalWorkerNodes: number;
  totalControlPlaneNodes: number;
}

/**
 * Determines cloud provider from node provider ID or labels
 */
function detectCloudProvider(node: ClusterNode): string | undefined {
  const providerID = node.spec?.providerID;

  if (providerID) {
    if (providerID.startsWith('aws://')) return 'aws';
    if (providerID.startsWith('gce://')) return 'gcp';
    if (providerID.startsWith('azure://')) return 'azure';
  }

  // Fallback to checking node labels and annotations
  const labels = node.metadata?.labels || {};
  const annotations = node.metadata?.annotations || {};

  // AWS
  if (labels['node.kubernetes.io/instance-type'] &&
      (annotations['node.alpha.kubernetes.io/provided-by-ec2'] ||
       labels['topology.kubernetes.io/zone']?.includes('amazonaws.com'))) {
    return 'aws';
  }

  // GCP
  if (labels['cloud.google.com/gke-nodepool'] ||
      labels['topology.kubernetes.io/zone']?.includes('gcp')) {
    return 'gcp';
  }

  // Azure
  if (labels['kubernetes.azure.com/cluster'] ||
      labels['topology.kubernetes.io/zone']?.includes('azure')) {
    return 'azure';
  }

  return undefined;
}

/**
 * Extracts instance type from node labels
 */
function getInstanceType(node: ClusterNode): string | undefined {
  return node.metadata?.labels?.['node.kubernetes.io/instance-type'];
}

/**
 * Determines node roles from labels
 */
function getNodeRoles(node: ClusterNode): string[] {
  const labels = node.metadata?.labels || {};
  const roles: string[] = [];

  if (labels['node-role.kubernetes.io/master'] !== undefined ||
      labels['node-role.kubernetes.io/control-plane'] !== undefined) {
    roles.push('master');
  }

  if (labels['node-role.kubernetes.io/worker'] !== undefined) {
    roles.push('worker');
  }

  // If no explicit worker label but no master label, assume worker
  if (roles.length === 0 || (!roles.includes('worker') && !roles.includes('master'))) {
    roles.push('worker');
  }

  return roles;
}

/**
 * Checks if node is ready
 */
function isNodeReady(node: ClusterNode): boolean {
  const conditions = node.status?.conditions || [];
  const readyCondition = conditions.find(condition => condition.type === 'Ready');
  return readyCondition?.status === 'True';
}

/**
 * Parses raw cluster node into our standardized format
 */
function parseClusterNode(node: ClusterNode): ParsedClusterNode {
  return {
    name: node.metadata.name,
    instanceType: getInstanceType(node),
    cloudProvider: detectCloudProvider(node),
    roles: getNodeRoles(node),
    ready: isNodeReady(node),
    // vcpus and memory will be populated by instance type mapping
  };
}

/**
 * Determines overall cluster cloud provider
 */
function determineClusterCloudProvider(nodes: ParsedClusterNode[]): ClusterInfo['cloudProvider'] {
  // Get cloud providers from all nodes
  const cloudProviders = nodes
    .map(node => node.cloudProvider)
    .filter(Boolean) as string[];

  // If no cloud provider detected, assume baremetal
  if (cloudProviders.length === 0) {
    return 'baremetal';
  }

  // Find the most common cloud provider
  const providerCounts: Record<string, number> = {};
  cloudProviders.forEach(provider => {
    providerCounts[provider] = (providerCounts[provider] || 0) + 1;
  });

  const dominantProvider = Object.keys(providerCounts).reduce((a, b) =>
    providerCounts[a] > providerCounts[b] ? a : b
  );

  // Validate against known providers
  if (['aws', 'gcp', 'azure'].includes(dominantProvider)) {
    return dominantProvider as 'aws' | 'gcp' | 'azure';
  }

  return 'unknown';
}

/**
 * Fetches cluster node information from Kubernetes API
 */
export async function fetchClusterInfo(): Promise<ClusterInfo> {
  try {
    // Query all nodes using OpenShift Console SDK
    const nodeModel = {
      apiVersion: 'v1',
      kind: 'Node',
      plural: 'nodes',
    };

    const [nodes] = await k8sList({
      model: nodeModel,
      queryParams: {},
    });

    // Parse nodes into our format
    const parsedNodes = (nodes as ClusterNode[]).map(parseClusterNode);

    // Separate by roles
    const workerNodes = parsedNodes.filter(node => node.roles.includes('worker'));
    const controlPlaneNodes = parsedNodes.filter(node => node.roles.includes('master'));

    // Determine cloud provider
    const cloudProvider = determineClusterCloudProvider(parsedNodes);

    const clusterInfo: ClusterInfo = {
      nodes: parsedNodes,
      cloudProvider,
      workerNodes,
      controlPlaneNodes,
      isBaremetal: cloudProvider === 'baremetal',
      totalWorkerNodes: workerNodes.length,
      totalControlPlaneNodes: controlPlaneNodes.length,
    };

    return clusterInfo;
  } catch (error) {
    console.error('Failed to fetch cluster information:', error);
    throw new Error(`Unable to query cluster information: ${error instanceof Error ? error.message : 'Unknown error'}`);
  }
}

/**
 * React hook for fetching cluster information
 * Returns [clusterInfo, loading, error]
 */
export function useClusterInfo(): [ClusterInfo | null, boolean, string | null] {
  const [clusterInfo, setClusterInfo] = useState<ClusterInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;

    const loadClusterInfo = async () => {
      try {
        setLoading(true);
        setError(null);
        const info = await fetchClusterInfo();
        if (mounted) {
          setClusterInfo(info);
        }
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Failed to load cluster information');
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };

    loadClusterInfo();

    return () => {
      mounted = false;
    };
  }, []);

  return [clusterInfo, loading, error];
}