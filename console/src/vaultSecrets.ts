import type { SecretFormData, SecretTemplate } from './types';

/**
 * Build values-secret.yaml and template JSON for the vault injection API,
 * from catalog template + user form values (same structure as vault_load_secrets / parse_secrets_info v2.0).
 */
export async function buildVaultInjectionYaml(
  secretTemplate: SecretTemplate,
  secretFormData: SecretFormData,
): Promise<{ valuesSecretYaml: string; templateYaml: string }> {
  const yaml = await import('js-yaml');

  const secretsList = secretTemplate.secrets.map((secretDef) => {
    const formValues = secretFormData[secretDef.name] || {};
    const secret: Record<string, unknown> = { name: secretDef.name };
    if (secretDef.vaultMount) secret.vaultMount = secretDef.vaultMount;
    if (secretDef.vaultPrefixes) secret.vaultPrefixes = secretDef.vaultPrefixes;
    secret.fields = secretDef.fields.map((fieldDef) => {
      const field: Record<string, unknown> = { name: fieldDef.name };
      if (fieldDef.onMissingValue) field.onMissingValue = fieldDef.onMissingValue;
      if (fieldDef.vaultPolicy) field.vaultPolicy = fieldDef.vaultPolicy;
      if (fieldDef.base64) field.base64 = fieldDef.base64;
      if (fieldDef.override) field.override = fieldDef.override;
      const val = formValues[fieldDef.name];
      if (typeof val === 'string' && val !== '') {
        field.value = val;
        if (fieldDef.onMissingValue === 'generate') {
          delete field.onMissingValue;
          delete field.vaultPolicy;
        }
      }
      return field;
    });
    return secret;
  });

  const vaultSecretStructure: SecretTemplate = {
    version: '2.0',
    secrets: secretsList as unknown as SecretTemplate['secrets'],
    vaultPolicies: secretTemplate?.vaultPolicies || null,
  };

  const valuesSecretYaml = yaml.dump(vaultSecretStructure);
  const templateYaml = JSON.stringify(secretTemplate, null, 2);
  return { valuesSecretYaml, templateYaml };
}
