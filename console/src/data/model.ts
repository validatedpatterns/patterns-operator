import {
  K8sGroupVersionKind,
  K8sResourceCommon,
} from '@openshift-console/dynamic-plugin-sdk';
import { K8sModel } from '@openshift-console/dynamic-plugin-sdk/lib/api/common-types';

export const patternManifestKind: K8sGroupVersionKind = {
  version: 'v1alpha1',
  group: 'gitops.hybrid-cloud-patterns.io',
  kind: 'PatternManifest',
};

export type PatternManifest = {
  // TODO: These fields probably shouldn't all be optional
  spec: {
    branch?: string;
    description?: string;
    maintainers?: string;
    products?: PatternManifestSpecProducts[];
    url?: string;
  };
} & K8sResourceCommon;

export type PatternManifestSpecProducts = {
  name: string;
};

export const PatternManifestModel: K8sModel = {
  apiVersion: patternManifestKind.version,
  apiGroup: patternManifestKind.group,
  label: patternManifestKind.kind,
  labelKey: patternManifestKind.kind,
  plural: 'patternmanifests',
  abbr: '',
  namespaced: true,
  crd: true,
  kind: patternManifestKind.kind,
  id: 'patternmanifest',
  labelPlural: 'PatternManifests',
  labelPluralKey: 'PatternManifest',
};
