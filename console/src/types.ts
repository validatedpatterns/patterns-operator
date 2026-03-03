export interface Catalog {
  generated_at: string;
  generator_version: string;
  patterns: string[];
}

export interface ComputeSpec {
  replicas: number;
  type: string;
}

export interface CloudRequirements {
  compute?: Record<string, ComputeSpec>;
  controlPlane?: Record<string, ComputeSpec>;
}

export interface Requirements {
  hub?: CloudRequirements;
  spoke?: CloudRequirements;
}

export interface ExtraFeatures {
  hypershift_support?: boolean;
  spoke_support?: boolean;
}

export interface Pattern {
  metadata_version: string;
  name: string;
  pattern_version: string;
  display_name: string;
  repo_url: string;
  docs_repo_url?: string;
  issues_url: string;
  docs_url: string;
  ci_url: string;
  tier: 'maintained' | 'tested' | 'sandbox';
  owners: string[];
  requirements?: Requirements;
  extra_features?: ExtraFeatures;
  external_requirements?: unknown;
  org: string;
  spoke?: unknown;
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
    [fieldName: string]: string | File | null;
  };
}
