import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useHistory, useRouteMatch } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Button,
  Card,
  CardBody,
  CardTitle,
  ExpandableSection,
  Form,
  FormGroup,
  PageSection,
  Spinner,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { k8sCreate } from '@openshift-console/dynamic-plugin-sdk';
import {
  fetchPattern,
  fetchSecretTemplate,
  fetchVaultJobStatus,
  triggerVaultInjection as apiTriggerVaultInjection,
  VaultJobStatus,
  VaultInjectionRequest
} from '../api';
import { SecretTemplate, SecretFormData, SecretDefinition, SecretField } from '../types';
import { GenerateField } from './SecretForm/GenerateField';
import { PromptField } from './SecretForm/PromptField';
import { FileField } from './SecretForm/FileField';
import { IniField } from './SecretForm/IniField';
import { StaticField } from './SecretForm/StaticField';
import './SecretForm/SecretForm.css';

const PatternModel = {
  apiGroup: 'gitops.hybrid-cloud-patterns.io',
  apiVersion: 'v1alpha1',
  kind: 'Pattern',
  abbr: 'P',
  label: 'Pattern',
  labelPlural: 'Patterns',
  plural: 'patterns',
  namespaced: true,
};

export default function InstallPatternPage() {
  const { t } = useTranslation('plugin__console-plugin-template');
  const history = useHistory();
  const match = useRouteMatch<{ name: string }>('/patterns/install/:name');
  const name = match?.params?.name;

  // Secret form state (integrated inline instead of separate page)

  const [loading, setLoading] = React.useState(true);
  const [fetchError, setFetchError] = React.useState<string | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  const [submitError, setSubmitError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState(false);

  const [patternName, setPatternName] = React.useState('');
  const [targetRepo, setTargetRepo] = React.useState('');
  const [targetRevision, setTargetRevision] = React.useState('main');

  const [secretTemplate, setSecretTemplate] = React.useState<SecretTemplate | null>(null);
  const [secretFormData, setSecretFormData] = React.useState<SecretFormData>({});
  const [expandedSections, setExpandedSections] = React.useState<Record<string, boolean>>({});
  const [vaultJobStatus, setVaultJobStatus] = React.useState<VaultJobStatus | null>(null);
  const [checkingVaultStatus, setCheckingVaultStatus] = React.useState(false);

  React.useEffect(() => {
    console.log('🔵 [InstallPatternPage] Starting to load pattern data for:', name);

    Promise.all([fetchPattern(name), fetchSecretTemplate(name)])
      .then(([patternData, template]) => {
        console.log('🟢 [InstallPatternPage] Pattern data loaded successfully:', {
          patternName: patternData.name,
          repoUrl: patternData.repo_url,
          hasSecretTemplate: !!template
        });

        setPatternName(patternData.name);
        setTargetRepo(patternData.repo_url || '');
        // Only use the template if it has actual secrets defined
        const hasSecrets = template && template.secrets && template.secrets.length > 0;
        setSecretTemplate(hasSecrets ? template : null);

        // Initialize secret form data if template has secrets
        if (hasSecrets) {
          console.log('🔧 [InstallPatternPage] Initializing secret form with template:', template);
          const initialData: SecretFormData = {};
          const initialExpanded: Record<string, boolean> = {};

          template.secrets.forEach((secret, index) => {
            console.log(`🔧 [InstallPatternPage] Processing secret: ${secret.name} with ${secret.fields.length} fields`);
            initialData[secret.name] = {};
            secret.fields.forEach((field) => {
              initialData[secret.name][field.name] = '';
            });
            // Expand the first section by default
            initialExpanded[secret.name] = index === 0;
          });

          console.log('🔧 [InstallPatternPage] Secret form initialized:', {
            secretCount: template.secrets.length,
            initialData: Object.keys(initialData),
            expandedSections: Object.keys(initialExpanded).filter(key => initialExpanded[key])
          });

          setSecretFormData(initialData);
          setExpandedSections(initialExpanded);
        }

        setLoading(false);
      })
      .catch((err) => {
        console.error('🔴 [InstallPatternPage] Failed to load pattern data:', err);
        setFetchError(err?.message || String(err));
        setLoading(false);
      });
  }, [name]);

  const triggerVaultInjection = React.useCallback(async () => {
    console.log('🚀 [InstallPatternPage] Starting vault injection process');

    if (!patternName || !secretFormData || !secretTemplate) {
      console.log('🟡 [InstallPatternPage] Missing required data for vault injection:', {
        patternName: !!patternName,
        secretFormData: !!secretFormData && Object.keys(secretFormData).length > 0,
        secretTemplate: !!secretTemplate,
      });
      return;
    }

    console.log('✅ [InstallPatternPage] All required data present for vault injection:', {
      patternName,
      secretDataKeys: Object.keys(secretFormData),
      templateSecrets: secretTemplate.secrets.map(s => s.name)
    });

    try {
      // Convert secretFormData to YAML format with proper structure for vault_load_secrets
      const yaml = await import('js-yaml');
      console.log('🔄 [InstallPatternPage] Converting secretFormData to YAML:', secretFormData);

      // Build the v2.0 secrets list structure expected by parse_secrets_info
      const secretsList = secretTemplate.secrets.map((secretDef) => {
        const formValues = secretFormData[secretDef.name] || {};
        const secret: any = { name: secretDef.name };
        if (secretDef.vaultMount) secret.vaultMount = secretDef.vaultMount;
        if (secretDef.vaultPrefixes) secret.vaultPrefixes = secretDef.vaultPrefixes;
        secret.fields = secretDef.fields.map((fieldDef) => {
          const field: any = { name: fieldDef.name };
          if (fieldDef.onMissingValue) field.onMissingValue = fieldDef.onMissingValue;
          if (fieldDef.vaultPolicy) field.vaultPolicy = fieldDef.vaultPolicy;
          if (fieldDef.base64) field.base64 = fieldDef.base64;
          if (fieldDef.override) field.override = fieldDef.override;
          const val = formValues[fieldDef.name];
          if (typeof val === 'string' && val !== '') {
            field.value = val;
            // User provided an explicit value, so don't auto-generate
            if (fieldDef.onMissingValue === 'generate') {
              delete field.onMissingValue;
              delete field.vaultPolicy;
            }
          }
          return field;
        });
        return secret;
      });

      const vaultSecretStructure = {
        version: '2.0',
        secrets: secretsList
      };

      const valuesSecretYaml = yaml.dump(vaultSecretStructure);
      const templateYaml = JSON.stringify(secretTemplate, null, 2);
      console.log('✅ [InstallPatternPage] Generated values YAML with vault structure:', valuesSecretYaml);
      console.log('✅ [InstallPatternPage] Generated template YAML:', templateYaml);

      const request: VaultInjectionRequest = {
        patternName,
        valuesSecretYaml,
        templateYaml,
      };

      console.log('🚀 [InstallPatternPage] Triggering vault injection with request:', request);
      const result = await apiTriggerVaultInjection(request);
      console.log('📥 [InstallPatternPage] Vault injection result:', result);

      if (result.success) {
        console.log('✅ [InstallPatternPage] Vault injection triggered successfully, starting job status polling');
        // Start polling for job status
        setTimeout(() => {
          checkVaultJobStatus();
        }, 2000);
      } else {
        console.error('🔴 [InstallPatternPage] Vault injection failed:', result.message);
        setVaultJobStatus({
          status: 'not-found',
          message: result.message,
        });
      }
    } catch (err) {
      console.error('🔴 [InstallPatternPage] Error triggering vault injection:', err);
      const errorMessage = err.message || err.toString();
      setVaultJobStatus({
        status: 'not-found',
        message: `Failed to trigger vault injection: ${errorMessage}`,
      });
    }
  }, [patternName, secretFormData, secretTemplate]);

  const checkVaultJobStatus = React.useCallback(async () => {
    if (!patternName) {
      console.log('🟡 [InstallPatternPage] No pattern name for vault job status check');
      return;
    }

    try {
      console.log('🔍 [InstallPatternPage] Checking vault job status for pattern:', patternName);
      setCheckingVaultStatus(true);
      const status = await fetchVaultJobStatus(patternName);
      console.log('📋 [InstallPatternPage] Vault job status received:', status);
      setVaultJobStatus(status);

      // Continue polling if job is still running or pending
      if (status.status === 'running' || status.status === 'pending') {
        console.log('⏳ [InstallPatternPage] Job still in progress, will poll again in 5 seconds');
        setTimeout(() => {
          checkVaultJobStatus();
        }, 5000); // Poll every 5 seconds
      } else {
        console.log('✅ [InstallPatternPage] Job finished with status:', status.status);
      }
    } catch (err) {
      console.error('🔴 [InstallPatternPage] Error checking vault job status:', err);
    } finally {
      setCheckingVaultStatus(false);
    }
  }, [patternName]);

  // Check vault job status on component mount if secrets were configured
  React.useEffect(() => {
    const hasSecretData = secretFormData && Object.keys(secretFormData).length > 0;
    if (success && hasSecretData && secretTemplate && patternName) {
      console.log('⏰ [InstallPatternPage] Pattern created successfully with secrets, starting vault job status check');
      const timer = setTimeout(() => {
        checkVaultJobStatus();
      }, 2000); // Wait 2 seconds after pattern creation
      return () => clearTimeout(timer);
    }
  }, [success, secretFormData, secretTemplate, patternName, checkVaultJobStatus]);

  const handleSubmit = async () => {
    console.log('🚀 [InstallPatternPage] Starting pattern installation process');
    setSubmitting(true);
    setSubmitError(null);

    try {
      const hasSecrets = secretFormData && Object.keys(secretFormData).length > 0 && secretTemplate;
      console.log('📊 [InstallPatternPage] Installation details:', {
        patternName,
        clusterGroupName: 'hub',
        targetRepo,
        targetRevision,
        hasSecrets,
        secretCount: hasSecrets ? Object.keys(secretFormData).length : 0
      });
      const patternData: {
        apiVersion: string;
        kind: string;
        metadata: { name: string; namespace: string };
        spec: {
          clusterGroupName: string;
          gitSpec: { targetRepo: string; targetRevision: string };
          secretsConfig?: { template: string; values: string };
        };
      } = {
        apiVersion: 'gitops.hybrid-cloud-patterns.io/v1alpha1',
        kind: 'Pattern',
        metadata: {
          name: patternName,
          // FIXME(bandini): we need a way to override this for the time when we move our operator to
          // another namespace
          namespace: 'openshift-operators',
        },
        spec: {
          clusterGroupName: 'hub',
          gitSpec: {
            targetRepo,
            targetRevision,
          },
        },
      };

      console.log('🔧 [InstallPatternPage] Creating Pattern CR with data:', JSON.stringify(patternData, null, 2));

      await k8sCreate({
        model: PatternModel,
        data: patternData,
      });

      console.log('✅ [InstallPatternPage] Pattern CR created successfully');
      setSuccess(true);

      // If secrets were configured, trigger vault injection
      if (hasSecrets) {
        console.log('🔐 [InstallPatternPage] Secrets detected, triggering vault injection');
        await triggerVaultInjection();
      } else {
        console.log('🔓 [InstallPatternPage] No secrets configured, skipping vault injection');
      }
    } catch (err) {
      console.error('🔴 [InstallPatternPage] Pattern installation failed:', err);
      setSubmitError(err?.message || String(err));
    } finally {
      setSubmitting(false);
      console.log('🏁 [InstallPatternPage] Pattern installation process finished');
    }
  };

  // Secret form handling functions
  const handleFieldChange = (
    secretName: string,
    fieldName: string,
    value: string | File | null,
  ) => {
    console.log(`🔄 [InstallPatternPage] Secret field changed: ${secretName}.${fieldName}`, { value: value instanceof File ? `[File: ${value.name}]` : value });
    setSecretFormData((prev) => ({
      ...prev,
      [secretName]: {
        ...prev[secretName],
        [fieldName]: value,
      },
    }));
  };

  const toggleSection = (sectionName: string) => {
    console.log(`🔄 [InstallPatternPage] Toggling secret section: ${sectionName}`);
    setExpandedSections((prev) => ({
      ...prev,
      [sectionName]: !prev[sectionName],
    }));
  };

  const getFieldType = (field: SecretField): 'generate' | 'prompt' | 'file' | 'ini' | 'static' => {
    if (field.onMissingValue === 'generate') return 'generate';
    if (field.path) return 'file';
    if (field.ini_file) return 'ini';
    if (field.value !== undefined && field.value !== null) return 'static';
    return 'prompt'; // Default to prompt for required fields
  };

  const renderField = (secret: SecretDefinition, field: SecretField) => {
    const fieldType = getFieldType(field);
    const value = secretFormData[secret.name]?.[field.name] || '';

    const commonProps = {
      field,
      value,
      onChange: (newValue: string | File | null) =>
        handleFieldChange(secret.name, field.name, newValue),
    };

    switch (fieldType) {
      case 'generate':
        return <GenerateField key={field.name} {...commonProps} />;
      case 'prompt':
        return <PromptField key={field.name} {...commonProps} />;
      case 'file':
        return <FileField key={field.name} {...commonProps} />;
      case 'ini':
        return <IniField key={field.name} {...commonProps} />;
      case 'static':
        return <StaticField key={field.name} {...commonProps} />;
      default:
        return <PromptField key={field.name} {...commonProps} />;
    }
  };

  if (loading) {
    return (
      <PageSection>
        <Spinner aria-label={t('Loading pattern')} />
      </PageSection>
    );
  }

  if (fetchError) {
    return (
      <PageSection>
        <Alert variant="danger" title={t('Failed to load pattern')}>
          {fetchError}
        </Alert>
      </PageSection>
    );
  }

  return (
    <>
      <Helmet>
        <title>{t('Install Pattern')}</title>
      </Helmet>
      <PageSection>
        <Title headingLevel="h1">{t('Install Pattern')}</Title>
      </PageSection>
      <PageSection>
        {success && (
          <Alert variant="success" title={t('Pattern created successfully')}>
            <p>{t('Your pattern has been created and ArgoCD will begin deploying it shortly.')}</p>
            <Button variant="link" onClick={() => history.push('/patterns')}>
              {t('Back to catalog')}
            </Button>
          </Alert>
        )}
        {/* Vault injection status */}
        {success && secretFormData && Object.keys(secretFormData).length > 0 && secretTemplate && vaultJobStatus && (
          <Alert
            variant={
              vaultJobStatus.status === 'succeeded'
                ? 'success'
                : vaultJobStatus.status === 'failed'
                ? 'danger'
                : 'info'
            }
            title={t('Vault Secret Injection')}
            isInline
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              {(vaultJobStatus.status === 'running' || vaultJobStatus.status === 'pending' || checkingVaultStatus) && (
                <Spinner aria-label={t('Checking vault status')} size="md" />
              )}
              <span>{vaultJobStatus.message}</span>
            </div>
            {vaultJobStatus.jobName && (
              <p style={{ marginTop: '8px', fontSize: '0.9em', color: 'var(--pf-v6-global--palette--black-700)' }}>
                {t('Job')}: <a href={`/k8s/ns/openshift-operators/jobs/${vaultJobStatus.jobName}`}><code>{vaultJobStatus.jobName}</code></a>
              </p>
            )}
          </Alert>
        )}
        {submitError && (
          <Alert variant="danger" title={t('Failed to create pattern')}>
            {submitError}
          </Alert>
        )}
        {!success && (
          <Form
            onSubmit={(e) => {
              e.preventDefault();
              handleSubmit();
            }}
          >
            <FormGroup label={t('Name')} isRequired fieldId="pattern-name">
              <TextInput
                id="pattern-name"
                isRequired
                value={patternName}
                onChange={(_event, value) => setPatternName(value)}
              />
            </FormGroup>
            <FormGroup label={t('Target Repo')} isRequired fieldId="pattern-target-repo">
              <TextInput
                id="pattern-target-repo"
                isRequired
                value={targetRepo}
                onChange={(_event, value) => setTargetRepo(value)}
              />
            </FormGroup>
            <FormGroup label={t('Target Revision')} isRequired fieldId="pattern-target-revision">
              <TextInput
                id="pattern-target-revision"
                isRequired
                value={targetRevision}
                onChange={(_event, value) => setTargetRevision(value)}
              />
            </FormGroup>

            {/* Secrets Configuration Section */}
            {secretTemplate && (
              <FormGroup label={t('Secrets Configuration')} fieldId="pattern-secrets">
                <Alert variant="info" title={t('Optional Secret Configuration')} isInline>
                  {t('Configure secrets that will be injected into Vault for this pattern.')}
                </Alert>
                <div className="patterns-operator__secret-form">
                  {secretTemplate.secrets.map((secret) => (
                    <ExpandableSection
                      key={secret.name}
                      toggleText={secret.name}
                      isExpanded={expandedSections[secret.name] || false}
                      onToggle={() => toggleSection(secret.name)}
                      className="patterns-operator__secret-section"
                    >
                      <Card>
                        <CardTitle>{secret.name}</CardTitle>
                        <CardBody>
                          {secret.fields.map((field) => (
                            <FormGroup
                              key={field.name}
                              label={field.name}
                              helperText={field.description}
                              isRequired={getFieldType(field) === 'prompt'}
                              fieldId={`secret-${secret.name}-${field.name}`}
                              className="patterns-operator__secret-field"
                            >
                              {renderField(secret, field)}
                            </FormGroup>
                          ))}
                        </CardBody>
                      </Card>
                    </ExpandableSection>
                  ))}
                </div>
              </FormGroup>
            )}

            <ActionGroup>
              <Button
                variant="primary"
                type="submit"
                isLoading={submitting}
                isDisabled={submitting}
              >
                {t('Install')}
              </Button>
              <Button variant="link" onClick={() => history.push('/patterns')}>
                {t('Cancel')}
              </Button>
            </ActionGroup>
          </Form>
        )}
      </PageSection>
    </>
  );
}
