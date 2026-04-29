import { consoleFetch } from '@openshift-console/dynamic-plugin-sdk';
import { load } from 'js-yaml';
import {
  Catalog,
  Pattern,
  SecretTemplate,
  VAULT_UPLOADS_MOUNT_PREFIX,
  type VaultInjectionFileArtifact,
} from './types';

declare const __PATTERN_UI_CATALOG_BASE_URL__: string;
declare const __PATTERN_OPERATOR_NS__: string;

const DEFAULT_PATTERN_OPERATOR_NS = 'patterns-operator';
export const PATTERN_OPERATOR_NS = __PATTERN_OPERATOR_NS__ || DEFAULT_PATTERN_OPERATOR_NS;

const DEFAULT_PATTERN_UI_CATALOG_BASE_URL =
  '/api/proxy/plugin/patterns-operator-console-plugin/pattern-ui-catalog';
const PATTERN_UI_CATALOG_BASE_URL =
  __PATTERN_UI_CATALOG_BASE_URL__ || DEFAULT_PATTERN_UI_CATALOG_BASE_URL;

async function fetchYAML<T>(url: string): Promise<T> {
  const response = await consoleFetch(url, { cache: 'no-store' });
  const text = await response.text();
  return load(text) as T;
}

export async function fetchCatalog(): Promise<Catalog> {
  return fetchYAML<Catalog>(`${PATTERN_UI_CATALOG_BASE_URL}/catalog.yaml`);
}

export async function fetchPattern(name: string): Promise<Pattern> {
  return fetchYAML<Pattern>(`${PATTERN_UI_CATALOG_BASE_URL}/${name}/pattern.yaml`);
}

export async function fetchCatalogImage(): Promise<string> {
  try {
    const response = await consoleFetch(
      `/api/kubernetes/apis/apps/v1/namespaces/${PATTERN_OPERATOR_NS}/deployments/patterns-operator-pattern-ui-catalog`,
    );
    const data = await response.json();
    const containers = data.spec?.template?.spec?.containers || [];
    const catalogContainer = containers.find(
      (c: any) => c.name === 'patterns-operator-pattern-ui-catalog',
    );
    return catalogContainer?.image || 'unknown';
  } catch (error) {
    return 'unknown';
  }
}

export async function fetchAllPatterns(): Promise<{
  patterns: Pattern[];
  catalogDescription?: string;
}> {
  const catalog = await fetchCatalog();
  const patterns = await Promise.all(
    catalog.patterns.map(async (key) => {
      const pattern = await fetchPattern(key);
      return { ...pattern, catalogKey: key };
    }),
  );
  return { patterns, catalogDescription: catalog.catalog_description };
}

export interface VaultJobStatus {
  jobName?: string;
  status: 'not-found' | 'pending' | 'running' | 'succeeded' | 'failed';
  message: string;
  conditions?: any[];
}

export interface VaultInjectionRequest {
  patternName: string;
  valuesSecretYaml: string;
  /** One Kubernetes Secret per entry, mounted under {@link VAULT_UPLOADS_MOUNT_PREFIX}/{slug}. */
  fileArtifacts?: VaultInjectionFileArtifact[];
  vaultNamespace?: string;
  vaultPod?: string;
  vaultHub?: string;
}

export interface VaultInjectionResponse {
  success: boolean;
  message: string;
  jobName?: string;
  secretName?: string;
}

export async function triggerVaultInjection(
  request: VaultInjectionRequest,
): Promise<VaultInjectionResponse> {
  try {
    console.log('🚀 [API] Starting vault injection for pattern:', request.patternName);
    const fileArtifacts = request.fileArtifacts ?? [];
    console.log('📊 [API] Request details:', {
      patternName: request.patternName,
      valuesSecretYamlLength: request.valuesSecretYaml?.length || 0,
      fileArtifactCount: fileArtifacts.length,
      vaultNamespace: request.vaultNamespace || 'vault',
      vaultPod: request.vaultPod || 'vault-0',
      vaultHub: request.vaultHub || 'hub',
    });

    const timestamp = Date.now();
    const secretName = `vault-secrets-${request.patternName}-${timestamp}`;
    const jobName = `vault-inject-${request.patternName}-${timestamp}`;
    const runLabelKey = 'patterns.gitops.hybrid-cloud-patterns.io/vault-injection-run';
    const runLabelValue = String(timestamp);

    const commonSecretLabels = {
      'patterns.gitops.hybrid-cloud-patterns.io/pattern': request.patternName,
      'patterns.gitops.hybrid-cloud-patterns.io/component': 'secret-injector',
      [runLabelKey]: runLabelValue,
    };

    console.log('🔐 [API] Creating Kubernetes secret:', secretName);
    console.log('⚙️ [API] Creating Kubernetes job:', jobName);

    for (let i = 0; i < fileArtifacts.length; i++) {
      const artifact = fileArtifacts[i];
      const fileSecretName = `${secretName}-f${i}`;
      const fileSecret = {
        apiVersion: 'v1',
        kind: 'Secret',
        metadata: {
          name: fileSecretName,
          namespace: PATTERN_OPERATOR_NS,
          labels: { ...commonSecretLabels },
        },
        type: 'Opaque',
        data: { content: artifact.dataBase64 },
      };

      const fileResp = await consoleFetch(
        `/api/kubernetes/api/v1/namespaces/${PATTERN_OPERATOR_NS}/secrets`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(fileSecret),
        },
      );

      if (!fileResp.ok) {
        const errorText = await fileResp.text();
        console.error(
          '🔴 [API] Failed to create file secret:',
          fileSecretName,
          fileResp.status,
          errorText,
        );
        throw new Error(`Failed to create file secret: ${fileResp.status} ${errorText}`);
      }
    }

    const secretData: Record<string, string> = {
      'values-secret.yaml': btoa(request.valuesSecretYaml),
    };

    const secret = {
      apiVersion: 'v1',
      kind: 'Secret',
      metadata: {
        name: secretName,
        namespace: PATTERN_OPERATOR_NS,
        labels: { ...commonSecretLabels },
      },
      type: 'Opaque',
      data: secretData,
    };

    console.log(
      '🔐 [API] Creating values-secret with payload size:',
      JSON.stringify(secret).length,
      'bytes',
    );

    const secretResponse = await consoleFetch(
      `/api/kubernetes/api/v1/namespaces/${PATTERN_OPERATOR_NS}/secrets`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(secret),
      },
    );

    if (!secretResponse.ok) {
      const errorText = await secretResponse.text();
      console.error('🔴 [API] Failed to create secret:', secretResponse.status, errorText);
      throw new Error(`Failed to create secret: ${secretResponse.status} ${errorText}`);
    }

    const secretResult = await secretResponse.json();
    console.log('✅ [API] Secret created successfully:', secretName);
    console.log('✅ [API] Secret creation result:', {
      name: secretResult.metadata?.name,
      uid: secretResult.metadata?.uid,
      creationTimestamp: secretResult.metadata?.creationTimestamp,
    });

    const volumes: Record<string, unknown>[] = [
      {
        name: 'vault-secrets',
        secret: { secretName },
      },
    ];
    if (fileArtifacts.length > 0) {
      volumes.push({
        name: 'vault-uploads',
        projected: {
          sources: fileArtifacts.map((art, i) => ({
            secret: {
              name: `${secretName}-f${i}`,
              items: [{ key: 'content', path: art.slug }],
            },
          })),
        },
      });
    }
    volumes.push({ name: 'shared', emptyDir: {} });

    const injectorVolumeMounts: Record<string, unknown>[] = [
      {
        name: 'vault-secrets',
        mountPath: '/vault-secrets',
        readOnly: true,
      },
    ];
    if (fileArtifacts.length > 0) {
      injectorVolumeMounts.push({
        name: 'vault-uploads',
        mountPath: VAULT_UPLOADS_MOUNT_PREFIX,
        readOnly: true,
      });
    }
    injectorVolumeMounts.push({
      name: 'shared',
      mountPath: '/shared',
    });

    const job = {
      apiVersion: 'batch/v1',
      kind: 'Job',
      metadata: {
        name: jobName,
        namespace: PATTERN_OPERATOR_NS,
        labels: {
          'patterns.gitops.hybrid-cloud-patterns.io/pattern': request.patternName,
          'patterns.gitops.hybrid-cloud-patterns.io/component': 'secret-injector',
          [runLabelKey]: runLabelValue,
        },
      },
      spec: {
        backoffLimit: 3,
        template: {
          metadata: {
            labels: {
              'patterns.gitops.hybrid-cloud-patterns.io/pattern': request.patternName,
              'patterns.gitops.hybrid-cloud-patterns.io/component': 'secret-injector',
              [runLabelKey]: runLabelValue,
            },
          },
          spec: {
            serviceAccountName: 'patterns-operator-secret-injector',
            restartPolicy: 'Never',
            initContainers: [
              {
                name: 'secret-injector',
                image: 'quay.io/validatedpatterns/imperative-container:v1',
                command: [
                  '/bin/bash',
                  '-c',
                  `
                  echo "Starting vault secret injection for pattern: ${request.patternName}"

                  # Create a simplified playbook that calls the vault_load_secrets module directly
                  cat > /tmp/vault_injection_playbook.yaml << 'PLAYBOOK_EOF'
---
- name: Inject secrets into Vault
  hosts: localhost
  connection: local
  gather_facts: false
  vars:
    ansible_python_interpreter: "{{ ansible_playbook_python }}"
    check_missing_secrets: false
    namespace: "{{ vault_ns }}"
    pod: "{{ vault_pod }}"
  tasks:
    - name: Load secrets into vault using rhvp.cluster_utils module
      ansible.builtin.include_role:
        name: rhvp.cluster_utils.load_secrets
PLAYBOOK_EOF
                  # Run the playbook and save the exit code
                  cd /pattern-home
                  ansible-playbook -v -i localhost, /tmp/vault_injection_playbook.yaml \\
                    -e pattern_dir="/tmp/pattern" \\
                    -e vault_ns="${request.vaultNamespace || 'vault'}" \\
                    -e vault_pod="${request.vaultPod || 'vault-0'}" \\
                    -e vault_hub="${request.vaultHub || 'hub'}"
                  rc=$?

                  echo $rc > /shared/rc
                  echo "Vault secret injection finished with exit code $rc"
                  exit 0
                  `,
                ],
                env: [
                  { name: 'PATTERN_NAME', value: request.patternName },
                  { name: 'VALUES_SECRET', value: '/vault-secrets/values-secret.yaml' },
                ],
                volumeMounts: injectorVolumeMounts,
                resources: {
                  requests: { cpu: '100m', memory: '256Mi' },
                  limits: { cpu: '500m', memory: '512Mi' },
                },
              },
              {
                name: 'cleanup',
                image: 'quay.io/validatedpatterns/imperative-container:v1',
                command: [
                  '/bin/bash',
                  '-c',
                  `echo "Deleting temporary secrets for injection run $VAULT_INJECTION_RUN"
                  kubectl delete secret -n "$SECRET_NAMESPACE" -l "${runLabelKey}=$VAULT_INJECTION_RUN,patterns.gitops.hybrid-cloud-patterns.io/pattern=$PATTERN_NAME" --ignore-not-found
                  echo "Cleanup complete"`,
                ],
                env: [
                  { name: 'SECRET_NAMESPACE', value: PATTERN_OPERATOR_NS },
                  { name: 'VAULT_INJECTION_RUN', value: runLabelValue },
                  { name: 'PATTERN_NAME', value: request.patternName },
                ],
                resources: {
                  requests: { cpu: '50m', memory: '64Mi' },
                  limits: { cpu: '100m', memory: '128Mi' },
                },
              },
            ],
            containers: [
              {
                name: 'result',
                image: 'quay.io/validatedpatterns/imperative-container:v1',
                command: [
                  '/bin/bash',
                  '-c',
                  `rc=$(cat /shared/rc 2>/dev/null || echo 1)
                  if [ "$rc" -eq 0 ]; then
                    echo "Vault secret injection completed successfully"
                  else
                    echo "Vault secret injection failed with exit code $rc"
                  fi
                  exit $rc`,
                ],
                volumeMounts: [
                  {
                    name: 'shared',
                    mountPath: '/shared',
                    readOnly: true,
                  },
                ],
                resources: {
                  requests: { cpu: '10m', memory: '16Mi' },
                  limits: { cpu: '50m', memory: '32Mi' },
                },
              },
            ],
            volumes,
          },
        },
      },
    };

    // Create the job
    console.log('⚙️ [API] Creating job with payload size:', JSON.stringify(job).length, 'bytes');
    console.log('⚙️ [API] Job metadata:', {
      name: job.metadata.name,
      namespace: job.metadata.namespace,
      labels: job.metadata.labels,
      serviceAccountName: job.spec.template.spec.serviceAccountName,
      containerImage: job.spec.template.spec.containers[0].image,
    });

    const jobResponse = await consoleFetch(
      `/api/kubernetes/apis/batch/v1/namespaces/${PATTERN_OPERATOR_NS}/jobs`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(job),
      },
    );

    if (!jobResponse.ok) {
      const errorText = await jobResponse.text();
      console.error('🔴 [API] Failed to create job:', jobResponse.status, errorText);
      throw new Error(`Failed to create job: ${jobResponse.status} ${errorText}`);
    }

    const jobData = await jobResponse.json();
    console.log('✅ [API] Job created successfully:', jobData.metadata?.name);
    console.log('✅ [API] Job creation result:', {
      name: jobData.metadata?.name,
      uid: jobData.metadata?.uid,
      creationTimestamp: jobData.metadata?.creationTimestamp,
      backoffLimit: jobData.spec?.backoffLimit,
    });

    console.log('🎉 [API] Vault injection setup completed successfully');
    return {
      success: true,
      message: 'Vault injection job created successfully',
      jobName,
      secretName,
    };
  } catch (error) {
    console.error('🔴 [API] Error triggering vault injection:', error);
    console.error('🔴 [API] Error details:', {
      name: error.name,
      message: error.message,
      stack: error.stack,
    });
    return {
      success: false,
      message: `Error triggering vault injection: ${error.message || error}`,
    };
  }
}

export async function fetchVaultJobStatus(patternName: string): Promise<VaultJobStatus> {
  try {
    const url = `/api/kubernetes/apis/batch/v1/namespaces/${PATTERN_OPERATOR_NS}/jobs?labelSelector=patterns.gitops.hybrid-cloud-patterns.io/pattern=${patternName},patterns.gitops.hybrid-cloud-patterns.io/component=secret-injector`;
    console.log(`🔍 [API] Fetching vault job status for pattern: ${patternName}`);
    console.log(`🔍 [API] Request URL: ${url}`);

    const response = await consoleFetch(url);
    if (!response.ok) {
      console.error(
        `🔴 [API] Failed to fetch vault job status: ${response.status} ${response.statusText}`,
      );
      throw new Error(`Failed to fetch vault job status: ${response.status}`);
    }

    const data = await response.json();
    console.log(`📋 [API] Jobs response received:`, {
      itemCount: data.items?.length || 0,
      items:
        data.items?.map((job) => ({
          name: job.metadata?.name,
          creationTimestamp: job.metadata?.creationTimestamp,
          status: job.status,
        })) || [],
    });

    if (!data.items || data.items.length === 0) {
      console.log(`🟡 [API] No vault injection jobs found for pattern: ${patternName}`);
      return {
        status: 'not-found',
        message: 'No vault injection job found for this pattern',
      };
    }

    // Get the most recent job
    const job = data.items[data.items.length - 1];
    const jobStatus = job.status || {};

    console.log(`📊 [API] Processing job status for: ${job.metadata?.name}`, {
      jobName: job.metadata?.name,
      creationTimestamp: job.metadata?.creationTimestamp,
      status: jobStatus,
      conditions:
        jobStatus.conditions?.map((c) => ({ type: c.type, status: c.status, reason: c.reason })) ||
        [],
    });

    let status: VaultJobStatus['status'] = 'pending';
    let message = 'Vault secrets injection is pending';

    if (jobStatus.succeeded && jobStatus.succeeded > 0) {
      status = 'succeeded';
      message = 'Vault secrets injection completed successfully';
      console.log(`✅ [API] Job succeeded: ${job.metadata?.name}`);
    } else if (jobStatus.failed && jobStatus.failed > 0) {
      status = 'failed';
      message = 'Vault secrets injection failed';
      console.log(`❌ [API] Job failed: ${job.metadata?.name}`);
    } else if (jobStatus.active && jobStatus.active > 0) {
      status = 'running';
      message = 'Vault secrets injection is in progress';
      console.log(`⏳ [API] Job running: ${job.metadata?.name}`);
    } else {
      console.log(`⏸️ [API] Job pending: ${job.metadata?.name}`);
    }

    const result = {
      jobName: job.metadata.name,
      status,
      message,
      conditions: jobStatus.conditions || [],
    };

    console.log(`📋 [API] Final job status result:`, result);
    return result;
  } catch (error) {
    console.error(`🔴 [API] Error fetching vault job status for pattern ${patternName}:`, error);
    console.error(`🔴 [API] Error details:`, {
      name: error.name,
      message: error.message,
      stack: error.stack,
    });
    return {
      status: 'not-found',
      message: `Error checking vault job status: ${error.message || error}`,
    };
  }
}

export async function fetchInstalledPatterns(): Promise<string[]> {
  const response = await consoleFetch(
    `/api/kubernetes/apis/gitops.hybrid-cloud-patterns.io/v1alpha1/namespaces/${PATTERN_OPERATOR_NS}/patterns`,
  );
  if (!response.ok) {
    throw new Error(`Failed to fetch installed patterns: ${response.status}`);
  }
  const data = await response.json();
  return (data.items || []).map((item: any) => item.metadata.name as string);
}

export interface PatternApplicationInfo {
  name: string;
  namespace: string;
  syncStatus: string;
  healthStatus: string;
  healthMessage?: string;
}

export interface PatternCRStatus {
  exists: boolean;
  lastStep?: string;
  lastError?: string;
  deletionPhase?: string;
  conditions?: any[];
  applications?: PatternApplicationInfo[];
  version?: number;
}

export async function fetchPatternCR(name: string): Promise<PatternCRStatus> {
  try {
    const response = await consoleFetch(
      `/api/kubernetes/apis/gitops.hybrid-cloud-patterns.io/v1alpha1/namespaces/${PATTERN_OPERATOR_NS}/patterns/${name}`,
    );
    if (!response.ok) {
      if (response.status === 404) {
        return { exists: false };
      }
      throw new Error(`Failed to fetch pattern CR: ${response.status}`);
    }
    const data = await response.json();
    const status = data.status || {};
    return {
      exists: true,
      lastStep: status.lastStep,
      lastError: status.lastError,
      deletionPhase: status.deletionPhase,
      conditions: status.conditions,
      applications: status.applications,
      version: status.version,
    };
  } catch (err) {
    // consoleFetch may throw on 404 instead of returning a response
    if (
      err?.response?.status === 404 ||
      err?.status === 404 ||
      (err?.message && /404|not found/i.test(err.message))
    ) {
      return { exists: false };
    }
    throw err;
  }
}

export async function deletePattern(name: string): Promise<void> {
  const response = await consoleFetch(
    `/api/kubernetes/apis/gitops.hybrid-cloud-patterns.io/v1alpha1/namespaces/${PATTERN_OPERATOR_NS}/patterns/${name}`,
    { method: 'DELETE' },
  );
  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Failed to delete pattern: ${response.status} ${errorText}`);
  }
}

export async function fetchSecretTemplate(name: string): Promise<SecretTemplate | null> {
  try {
    return await fetchYAML<SecretTemplate>(
      `${PATTERN_UI_CATALOG_BASE_URL}/${name}/values-secret.yaml.template`,
    );
  } catch {
    return null; // Template doesn't exist
  }
}
