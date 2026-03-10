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
    console.log('🚀 [API] Starting vault injection for pattern:', request.patternName);
    console.log('📊 [API] Request details:', {
      patternName: request.patternName,
      valuesSecretYamlLength: request.valuesSecretYaml?.length || 0,
      hasTemplate: !!request.templateYaml,
      vaultNamespace: request.vaultNamespace || 'vault',
      vaultPod: request.vaultPod || 'vault-0',
      vaultHub: request.vaultHub || 'hub'
    });

    const timestamp = Date.now();
    const secretName = `vault-secrets-${request.patternName}-${timestamp}`;
    const jobName = `vault-inject-${request.patternName}-${timestamp}`;

    console.log('🔐 [API] Creating Kubernetes secret:', secretName);
    console.log('⚙️ [API] Creating Kubernetes job:', jobName);

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
          'patterns.gitops.hybrid-cloud-patterns.io/component': 'secret-injector',
        },
      },
      type: 'Opaque',
      data: secretData,
    };

    // Create the secret
    console.log('🔐 [API] Creating secret with payload size:', JSON.stringify(secret).length, 'bytes');
    console.log('🔐 [API] Secret metadata:', {
      name: secret.metadata.name,
      namespace: secret.metadata.namespace,
      labels: secret.metadata.labels
    });

    const secretResponse = await consoleFetch('/api/kubernetes/api/v1/namespaces/openshift-operators/secrets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(secret),
    });

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
      creationTimestamp: secretResult.metadata?.creationTimestamp
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
          'patterns.gitops.hybrid-cloud-patterns.io/component': 'secret-injector',
        },
      },
      spec: {
        backoffLimit: 3,
        template: {
          metadata: {
            labels: {
              'patterns.gitops.hybrid-cloud-patterns.io/pattern': request.patternName,
              'patterns.gitops.hybrid-cloud-patterns.io/component': 'secret-injector',
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

                  # Copy mounted secret files to expected location
                  mkdir -p /tmp/pattern
                  cp /vault-secrets/values-secret.yaml /tmp/pattern/values-secret.yaml
                  if [[ -f /vault-secrets/values-secret.yaml.template ]]; then
                    cp /vault-secrets/values-secret.yaml.template /tmp/pattern/values-secret.yaml.template
                  fi

                  echo "Secret files prepared, running ansible to inject into vault..."

                  # Create a simplified playbook that calls the vault_load_secrets module directly
                  cat > /tmp/vault_injection_playbook.yaml << 'PLAYBOOK_EOF'
---
- name: Inject secrets into Vault
  hosts: localhost
  connection: local
  gather_facts: false
  vars:
    ansible_python_interpreter: "{{ ansible_playbook_python }}"
    values_secrets: "{{ pattern_dir }}/values-secret.yaml"
    check_missing_secrets: false
    namespace: "{{ vault_ns }}"
    pod: "{{ vault_pod }}"
  tasks:
    - name: Check if values-secret.yaml.template exists
      ansible.builtin.stat:
        path: "{{ pattern_dir }}/values-secret.yaml.template"
      register: template_file_check

    - name: Set values-secret.yaml.template exists fact
      ansible.builtin.set_fact:
        values_secret_template: "{{ (template_file_check.stat.exists) | ternary(pattern_dir + '/values-secret.yaml.template', omit) }}"

    - name: Load secrets into vault using rhvp.cluster_utils module
      ansible.builtin.include_role:
        name: rhvp.cluster_utils.load_secrets
PLAYBOOK_EOF

                  # Run the playbook and save the exit code
                  cd /pattern-home
                  ansible-playbook -v -i localhost, /tmp/vault_injection_playbook.yaml \\
                    -e pattern_name="${request.patternName}" \\
                    -e pattern_dir="/tmp/pattern" \\
                    -e vault_ns="${request.vaultNamespace || 'vault'}" \\
                    -e vault_pod="${request.vaultPod || 'vault-0'}" \\
                    -e vault_hub="${request.vaultHub || 'hub'}" \\
                    -e found_file="/tmp/pattern/values-secret.yaml" \\
                    -e secret_template="/tmp/pattern/values-secret.yaml.template"
                  rc=$?

                  echo $rc > /shared/rc
                  echo "Vault secret injection finished with exit code $rc"
                  exit 0
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
                  {
                    name: 'shared',
                    mountPath: '/shared',
                  },
                ],
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
                  `echo "Deleting temporary secret $SECRET_NAME"
                  kubectl delete secret "$SECRET_NAME" -n openshift-operators --ignore-not-found
                  echo "Cleanup complete"`,
                ],
                env: [
                  { name: 'SECRET_NAME', value: secretName },
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
            volumes: [
              {
                name: 'vault-secrets',
                secret: { secretName: secretName },
              },
              {
                name: 'shared',
                emptyDir: {},
              },
            ],
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
      containerImage: job.spec.template.spec.containers[0].image
    });

    const jobResponse = await consoleFetch('/api/kubernetes/apis/batch/v1/namespaces/openshift-operators/jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(job),
    });

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
      backoffLimit: jobData.spec?.backoffLimit
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
      stack: error.stack
    });
    return {
      success: false,
      message: `Error triggering vault injection: ${error.message || error}`,
    };
  }
}

export async function fetchVaultJobStatus(patternName: string): Promise<VaultJobStatus> {
  try {
    const url = `/api/kubernetes/apis/batch/v1/namespaces/openshift-operators/jobs?labelSelector=patterns.gitops.hybrid-cloud-patterns.io/pattern=${patternName},patterns.gitops.hybrid-cloud-patterns.io/component=secret-injector`;
    console.log(`🔍 [API] Fetching vault job status for pattern: ${patternName}`);
    console.log(`🔍 [API] Request URL: ${url}`);

    const response = await consoleFetch(url);
    if (!response.ok) {
      console.error(`🔴 [API] Failed to fetch vault job status: ${response.status} ${response.statusText}`);
      throw new Error(`Failed to fetch vault job status: ${response.status}`);
    }

    const data = await response.json();
    console.log(`📋 [API] Jobs response received:`, {
      itemCount: data.items?.length || 0,
      items: data.items?.map(job => ({
        name: job.metadata?.name,
        creationTimestamp: job.metadata?.creationTimestamp,
        status: job.status
      })) || []
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
      conditions: jobStatus.conditions?.map(c => ({ type: c.type, status: c.status, reason: c.reason })) || []
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
      stack: error.stack
    });
    return {
      status: 'not-found',
      message: `Error checking vault job status: ${error.message || error}`,
    };
  }
}

export async function fetchInstalledPatterns(): Promise<string[]> {
  const response = await consoleFetch(
    '/api/kubernetes/apis/gitops.hybrid-cloud-patterns.io/v1alpha1/namespaces/openshift-operators/patterns',
  );
  if (!response.ok) {
    throw new Error(`Failed to fetch installed patterns: ${response.status}`);
  }
  const data = await response.json();
  return (data.items || []).map((item: any) => item.metadata.name as string);
}

export async function deletePattern(name: string): Promise<void> {
  const response = await consoleFetch(
    `/api/kubernetes/apis/gitops.hybrid-cloud-patterns.io/v1alpha1/namespaces/openshift-operators/patterns/${name}`,
    { method: 'DELETE' },
  );
  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(`Failed to delete pattern: ${response.status} ${errorText}`);
  }
}

export async function fetchSecretTemplate(name: string): Promise<SecretTemplate | null> {
  try {
    return await fetchYAML<SecretTemplate>(`${PROXY_BASE}/${name}/values-secret.yaml.template`);
  } catch {
    return null; // Template doesn't exist
  }
}
