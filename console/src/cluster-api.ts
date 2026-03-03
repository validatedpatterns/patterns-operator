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
  // Temporary fallback for debugging - can be enabled via localStorage
  if (typeof window !== 'undefined' && window.localStorage?.getItem('PATTERNS_DEBUG_SKIP_CLUSTER_CHECK') === 'true') {
    console.warn('SKIPPING cluster compatibility check due to debug flag');
    return {
      nodes: [],
      cloudProvider: 'unknown',
      workerNodes: [],
      controlPlaneNodes: [],
      isBaremetal: false,
      totalWorkerNodes: 0,
      totalControlPlaneNodes: 0,
    };
  }
  try {
    // Query all nodes using OpenShift Console SDK
    // Try different model formats to ensure compatibility
    const nodeModel = {
      apiVersion: 'v1',
      kind: 'Node',
      plural: 'nodes',
      namespaced: false, // Nodes are cluster-scoped
    };

    console.log('Fetching cluster nodes with model:', nodeModel);

    const result = await k8sList({
      model: nodeModel,
      queryParams: {},
    });

    // Debug: Log the actual result structure
    console.log('k8sList result structure:', {
      type: typeof result,
      isArray: Array.isArray(result),
      length: Array.isArray(result) ? result.length : 'N/A',
      keys: typeof result === 'object' && result ? Object.keys(result) : 'N/A'
    });

    // Handle different possible return formats from k8sList
    let nodes, loaded, error;

    if (Array.isArray(result)) {
      // Standard tuple format: [resources, loaded, error]
      [nodes, loaded, error] = result;
    } else if (result && typeof result === 'object') {
      // Alternative object format
      nodes = result.items || result.data || result;
      loaded = result.loaded !== false; // Default to true if not specified
      error = result.error;
    } else {
      // Direct array or other format
      nodes = result;
      loaded = true;
      error = null;
    }

    console.log('Extracted values:', {
      nodesType: typeof nodes,
      nodesIsArray: Array.isArray(nodes),
      nodesLength: Array.isArray(nodes) ? nodes.length : 'N/A',
      loaded,
      error
    });

    if (error) {
      throw new Error(`Kubernetes API error: ${error.message || error}`);
    }

    if (!loaded) {
      throw new Error('Failed to load node information from Kubernetes API');
    }

    // Ensure nodes is an array
    if (!Array.isArray(nodes)) {
      console.error('Final nodes value is not an array:', nodes);
      throw new Error('Invalid response format from Kubernetes API - nodes data is not an array');
    }

    // Handle empty nodes array
    if (nodes.length === 0) {
      console.warn('No nodes found in cluster');
      // Return minimal cluster info for empty cluster
      return {
        nodes: [],
        cloudProvider: 'unknown',
        workerNodes: [],
        controlPlaneNodes: [],
        isBaremetal: false,
        totalWorkerNodes: 0,
        totalControlPlaneNodes: 0,
      };
    }

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

    // Check for common permission-related error patterns
    if (error instanceof Error) {
      const errorMessage = error.message.toLowerCase();

      if (errorMessage.includes('forbidden') || errorMessage.includes('unauthorized')) {
        throw new Error('Permission denied: Console plugin does not have permission to list cluster nodes. Please ensure RBAC is configured correctly.');
      }

      if (errorMessage.includes('map is not a function')) {
        console.error('Detected array mapping issue - k8sList returned unexpected format');
        throw new Error('Cluster node data format issue - please check console logs and ensure proper cluster permissions');
      }

      if (errorMessage.includes('not found') && errorMessage.includes('node')) {
        throw new Error('Node resource not found - ensure the cluster has worker nodes and the API is accessible');
      }

      if (errorMessage.includes('kubernetes api error')) {
        throw new Error(`Cluster API access denied or unavailable: ${error.message}`);
      }

      if (errorMessage.includes('invalid response format')) {
        throw new Error(`Unexpected cluster data format: ${error.message}`);
      }

      if (errorMessage.includes('failed to load')) {
        throw new Error('Failed to load cluster node information - check console plugin RBAC permissions');
      }

      // Generic error with context
      throw new Error(`Unable to query cluster information: ${error.message}`);
    } else {
      throw new Error(`Unable to query cluster information: ${String(error)}`);
    }
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