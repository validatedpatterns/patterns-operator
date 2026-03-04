import { consoleFetch } from '@openshift-console/dynamic-plugin-sdk';
import { load } from 'js-yaml';
import { Catalog, Pattern, SecretTemplate } from './types';

const PROXY_BASE = '/api/proxy/plugin/patterns-operator-console-plugin/pattern-catalog';

async function fetchYAML<T>(url: string): Promise<T> {
  const response = await consoleFetch(url);
  const text = await response.text();
  return load(text) as T;
}

export async function fetchCatalog(): Promise<Catalog> {
  return fetchYAML<Catalog>(`${PROXY_BASE}/catalog.yaml`);
}

export async function fetchPattern(name: string): Promise<Pattern> {
  return fetchYAML<Pattern>(`${PROXY_BASE}/${name}/pattern.yaml`);
}

export async function fetchAllPatterns(): Promise<Pattern[]> {
  const catalog = await fetchCatalog();
  const patterns = await Promise.all(
    catalog.patterns.map(async (key) => {
      const pattern = await fetchPattern(key);
      return { ...pattern, catalogKey: key };
    }),
  );
  return patterns;
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
  templateYaml?: string;
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

export async function triggerVaultInjection(request: VaultInjectionRequest): Promise<VaultInjectionResponse> {
  try {
    const timestamp = Date.now();
    const secretName = `vault-secrets-${request.patternName}-${timestamp}`;
    const jobName = `vault-inject-${request.patternName}-${timestamp}`;

    // First, create a Secret with the values-secret.yaml content
    const secretData: any = {
      'values-secret.yaml': btoa(request.valuesSecretYaml), // base64 encode for Kubernetes
    };

    if (request.templateYaml) {
      secretData['values-secret.yaml.template'] = btoa(request.templateYaml);
    }

    const secret = {
      apiVersion: 'v1',
      kind: 'Secret',
      metadata: {
        name: secretName,
        namespace: 'openshift-operators',
        labels: {
          'patterns.gitops.hybrid-cloud-patterns.io/pattern': request.patternName,
          'patterns.gitops.hybrid-cloud-patterns.io/component': 'vault-injector',
        },
      },
      type: 'Opaque',
      data: secretData,
    };

    // Create the secret
    await consoleFetch('/api/kubernetes/api/v1/namespaces/openshift-operators/secrets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(secret),
    });

    // Now create a Job that uses this secret
    const job = {
      apiVersion: 'batch/v1',
      kind: 'Job',
      metadata: {
        name: jobName,
        namespace: 'openshift-operators',
        labels: {
          'patterns.gitops.hybrid-cloud-patterns.io/pattern': request.patternName,
          'patterns.gitops.hybrid-cloud-patterns.io/component': 'vault-injector',
        },
      },
      spec: {
        backoffLimit: 3,
        template: {
          metadata: {
            labels: {
              'patterns.gitops.hybrid-cloud-patterns.io/pattern': request.patternName,
              'patterns.gitops.hybrid-cloud-patterns.io/component': 'vault-injector',
            },
          },
          spec: {
            serviceAccountName: 'vault-injector',
            restartPolicy: 'Never',
            containers: [
              {
                name: 'vault-injector',
                image: 'quay.io/validatedpatterns/imperative-container:v1',
                command: [
                  '/bin/bash',
                  '-c',
                  `
                  set -e
                  echo "Starting vault secret injection for pattern: ${request.patternName}"

                  # Copy mounted secret files to expected location
                  mkdir -p /tmp/pattern
                  cp /vault-secrets/values-secret.yaml /tmp/pattern/values-secret.yaml
                  if [[ -f /vault-secrets/values-secret.yaml.template ]]; then
                    cp /vault-secrets/values-secret.yaml.template /tmp/pattern/values-secret.yaml.template
                  fi

                  echo "Secret files prepared, running ansible to inject into vault..."

                  # Run the same ansible role that CLI uses
                  cd /pattern-home
                  ansible-playbook -v -i localhost, /usr/share/ansible/collections/ansible_collections/rhvp/cluster_utils/roles/vault_utils/tasks/push_secrets.yaml \\
                    -e pattern_name="${request.patternName}" \\
                    -e pattern_dir="/tmp/pattern" \\
                    -e vault_ns="${request.vaultNamespace || 'vault'}" \\
                    -e vault_pod="${request.vaultPod || 'vault-0'}" \\
                    -e vault_hub="${request.vaultHub || 'hub'}" \\
                    -e found_file="/tmp/pattern/values-secret.yaml" \\
                    -e is_encrypted=false \\
                    -e secret_template="/tmp/pattern/values-secret.yaml.template"

                  echo "Vault secret injection completed successfully"
                  `,
                ],
                env: [
                  { name: 'PATTERN_NAME', value: request.patternName },
                  { name: 'VAULT_NAMESPACE', value: request.vaultNamespace || 'vault' },
                  { name: 'VAULT_POD', value: request.vaultPod || 'vault-0' },
                  { name: 'VAULT_HUB', value: request.vaultHub || 'hub' },
                ],
                volumeMounts: [
                  {
                    name: 'vault-secrets',
                    mountPath: '/vault-secrets',
                    readOnly: true,
                  },
                ],
                resources: {
                  requests: { cpu: '100m', memory: '256Mi' },
                  limits: { cpu: '500m', memory: '512Mi' },
                },
              },
            ],
            volumes: [
              {
                name: 'vault-secrets',
                secret: { secretName: secretName },
              },
            ],
          },
        },
      },
    };

    // Create the job
    await consoleFetch('/api/kubernetes/apis/batch/v1/namespaces/openshift-operators/jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(job),
    });

    return {
      success: true,
      message: 'Vault injection job created successfully',
      jobName,
      secretName,
    };
  } catch (error) {
    console.error('Error triggering vault injection:', error);
    return {
      success: false,
      message: `Error triggering vault injection: ${error.message || error}`,
    };
  }
}

export async function fetchVaultJobStatus(patternName: string): Promise<VaultJobStatus> {
  try {
    const url = `/api/kubernetes/apis/batch/v1/namespaces/openshift-operators/jobs?labelSelector=patterns.gitops.hybrid-cloud-patterns.io/pattern=${patternName},patterns.gitops.hybrid-cloud-patterns.io/component=vault-injector`;
    const response = await consoleFetch(url);
    const data = await response.json();

    if (!data.items || data.items.length === 0) {
      return {
        status: 'not-found',
        message: 'No vault injection job found for this pattern',
      };
    }

    // Get the most recent job
    const job = data.items[data.items.length - 1];
    const jobStatus = job.status || {};

    let status: VaultJobStatus['status'] = 'pending';
    let message = 'Vault secrets injection is pending';

    if (jobStatus.succeeded && jobStatus.succeeded > 0) {
      status = 'succeeded';
      message = 'Vault secrets injection completed successfully';
    } else if (jobStatus.failed && jobStatus.failed > 0) {
      status = 'failed';
      message = 'Vault secrets injection failed';
    } else if (jobStatus.active && jobStatus.active > 0) {
      status = 'running';
      message = 'Vault secrets injection is in progress';
    }

    return {
      jobName: job.metadata.name,
      status,
      message,
      conditions: jobStatus.conditions || [],
    };
  } catch (error) {
    console.error('Error fetching vault job status:', error);
    return {
      status: 'not-found',
      message: `Error checking vault job status: ${error.message || error}`,
    };
  }
}

export async function fetchSecretTemplate(name: string): Promise<SecretTemplate | null> {
  try {
    return await fetchYAML<SecretTemplate>(`${PROXY_BASE}/${name}/values-secret.yaml.template`);
  } catch {
    return null; // Template doesn't exist
  }
}
