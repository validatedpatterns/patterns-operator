export interface Catalog {
  generated_at: string;
  generator_version: string;
  catalog_description?: string;
  patterns: string[];
}

export interface ExtraFeatures {
  hypershift_support?: boolean;
  spoke_support?: boolean;
}

export interface NodeRequirement {
  replicas: number;
  type: string;
}

export type CloudRequirements = Record<string, NodeRequirement>;

export interface ClusterRoleRequirements {
  compute?: CloudRequirements;
  controlPlane?: CloudRequirements;
}

export type PatternRequirements = Record<string, ClusterRoleRequirements>;

export interface ExternalRequirements {
  cluster_sizing_note?: string;
}

export interface Pattern {
  metadata_version: string;
  name: string;
  pattern_version: string;
  display_name: string;
  repo_url: string;
  issues_url: string;
  docs_url: string;
  ci_url: string;
  tier: 'maintained' | 'tested' | 'sandbox';
  owners: string[];
  org: string;
  clustergroupname: string;
  description?: string;
  docs_repo_url?: string;
  spoke?: unknown;
  requirements?: PatternRequirements;
  extra_features?: ExtraFeatures;
  external_requirements?: ExternalRequirements;
  /** The catalog directory key used to fetch this pattern. */
  catalogKey?: string;
}

export interface SecretTemplate {
  version: string;
  backingStore?: string;
  vaultPolicies?: Record<string, string>;
  secrets: SecretDefinition[];
}

export interface SecretDefinition {
  name: string;
  vaultMount?: string;
  vaultPrefixes?: string[];
  fields: SecretField[];
}

export interface SecretField {
  name: string;
  onMissingValue?: 'generate' | 'prompt' | 'error';
  vaultPolicy?: string;
  value?: string | null;
  description?: string;
  path?: string;
  base64?: boolean;
  ini_file?: string;
  ini_section?: string;
  ini_key?: string;
  prompt?: string;
  override?: boolean;
}

export interface SecretFormData {
  [secretName: string]: {
    [fieldName: string]: string | null;
  };
}

/** One user-uploaded file; becomes a dedicated Secret mounted under {@link VAULT_UPLOADS_MOUNT_PREFIX}. */
export interface VaultInjectionFileArtifact {
  /** Path segment under the uploads mount (unique per secret+field). */
  slug: string;
  /** Raw file bytes, base64-encoded (valid for Kubernetes Secret `data`). */
  dataBase64: string;
}

export interface VaultInjectionPayload {
  valuesSecretYaml: string;
  fileArtifacts: VaultInjectionFileArtifact[];
}

/** Mount path in the vault injection Job pod; must match paths emitted in values-secret.yaml for file fields. */
export const VAULT_UPLOADS_MOUNT_PREFIX = '/vault-uploads';
