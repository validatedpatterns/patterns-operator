import type {
  SecretFormData,
  SecretTemplate,
  SecretField,
  SecretDefinition,
  VaultInjectionPayload,
  VaultInjectionFileArtifact,
} from './types';
import { VAULT_UPLOADS_MOUNT_PREFIX } from './types';
import { Document } from 'yaml';

function isFileTemplateField(fieldDef: SecretField): boolean {
  return Boolean(fieldDef.path);
}

/** Slug used as relative path under {@link VAULT_UPLOADS_MOUNT_PREFIX} (projected volume `path`). */
function fileUploadSlug(secretName: string, fieldName: string): string {
  const raw = `${secretName}_${fieldName}`;
  const slug = raw
    .toLowerCase()
    .replace(/[^a-z0-9_.-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 200);
  return slug || 'upload';
}

/**
 * Build values-secret.yaml plus file artifacts for separate Kubernetes Secrets,
 * from catalog template + user form values (same structure as vault_load_secrets / parse_secrets_info v2.0).
 */
export function buildVaultInjectionPayload(
  secretTemplate: SecretTemplate,
  secretFormData: SecretFormData,
): VaultInjectionPayload {
  const fileArtifacts: VaultInjectionFileArtifact[] = [];

  const secretsList = secretTemplate.secrets.map((secretDef) => {
    const formValues = secretFormData[secretDef.name] || {};
    const secret: SecretDefinition = { name: secretDef.name, fields: [] };
    if (secretDef.vaultMount) secret.vaultMount = secretDef.vaultMount;
    if (secretDef.vaultPrefixes) secret.vaultPrefixes = secretDef.vaultPrefixes;
    secret.fields = secretDef.fields.map((fieldDef) => {
      const field: SecretField = { name: fieldDef.name };
      if (fieldDef.onMissingValue) field.onMissingValue = fieldDef.onMissingValue;
      if (fieldDef.vaultPolicy) field.vaultPolicy = fieldDef.vaultPolicy;
      if (fieldDef.override) field.override = fieldDef.override;
      const val = formValues[fieldDef.name];
      const hasVal = val !== null && val !== undefined && val !== '';

      if (isFileTemplateField(fieldDef) && hasVal) {
        const slug = fileUploadSlug(secretDef.name, fieldDef.name);
        field.path = `${VAULT_UPLOADS_MOUNT_PREFIX}/${slug}`;
        field.base64 = true;
        fileArtifacts.push({ slug, dataBase64: val as string });
        return field;
      }

      if (hasVal) {
        field.value = val as string;
        if (fieldDef.onMissingValue === 'generate') {
          delete field.onMissingValue;
          delete field.vaultPolicy;
        }
      }
      return field;
    });
    return secret;
  });

  const vaultPolicies = secretTemplate.vaultPolicies;
  const hasVaultPolicies =
    vaultPolicies != null &&
    typeof vaultPolicies === 'object' &&
    Object.keys(vaultPolicies).length > 0;

  const vaultSecretStructure: SecretTemplate = {
    version: '2.0',
    secrets: secretsList,
    ...(hasVaultPolicies ? { vaultPolicies } : {}),
  };
  const doc = new Document(vaultSecretStructure);

  const valuesSecretYaml = doc.toString({ lineWidth: 0 });

  return { valuesSecretYaml, fileArtifacts };
}
