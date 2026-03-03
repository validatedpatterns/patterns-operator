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

export async function fetchSecretTemplate(name: string): Promise<SecretTemplate | null> {
  try {
    return await fetchYAML<SecretTemplate>(`${PROXY_BASE}/${name}/values-secret.yaml.template`);
  } catch {
    return null; // Template doesn't exist
  }
}
