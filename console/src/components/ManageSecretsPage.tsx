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
  Title,
} from '@patternfly/react-core';
import {
  fetchPattern,
  fetchSecretTemplate,
  fetchVaultJobStatus,
  triggerVaultInjection as apiTriggerVaultInjection,
  VaultJobStatus,
  VaultInjectionRequest,
} from '../api';
import { SecretTemplate, SecretFormData, SecretDefinition, SecretField } from '../types';
import { GenerateField } from './SecretForm/GenerateField';
import { PromptField } from './SecretForm/PromptField';
import { FileField } from './SecretForm/FileField';
import { IniField } from './SecretForm/IniField';
import { StaticField } from './SecretForm/StaticField';
import './SecretForm/SecretForm.css';

export default function ManageSecretsPage() {
  const { t } = useTranslation('plugin__console-plugin-template');
  const history = useHistory();
  const match = useRouteMatch<{ name: string }>('/patterns/secrets/:name');
  const name = match?.params?.name;

  const [loading, setLoading] = React.useState(true);
  const [fetchError, setFetchError] = React.useState<string | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  const [submitError, setSubmitError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState(false);

  const [patternName, setPatternName] = React.useState('');
  const [displayName, setDisplayName] = React.useState('');
  const [secretTemplate, setSecretTemplate] = React.useState<SecretTemplate | null>(null);
  const [secretFormData, setSecretFormData] = React.useState<SecretFormData>({});
  const [expandedSections, setExpandedSections] = React.useState<Record<string, boolean>>({});
  const [vaultJobStatus, setVaultJobStatus] = React.useState<VaultJobStatus | null>(null);
  const [checkingVaultStatus, setCheckingVaultStatus] = React.useState(false);

  React.useEffect(() => {
    Promise.all([fetchPattern(name), fetchSecretTemplate(name)])
      .then(([patternData, template]) => {
        setPatternName(patternData.name);
        setDisplayName(patternData.display_name || patternData.name);
        const hasSecrets = template && template.secrets && template.secrets.length > 0;
        setSecretTemplate(hasSecrets ? template : null);

        if (hasSecrets) {
          const initialData: SecretFormData = {};
          const initialExpanded: Record<string, boolean> = {};

          template.secrets.forEach((secret, index) => {
            initialData[secret.name] = {};
            secret.fields.forEach((field) => {
              initialData[secret.name][field.name] = '';
            });
            initialExpanded[secret.name] = index === 0;
          });

          setSecretFormData(initialData);
          setExpandedSections(initialExpanded);
        }

        setLoading(false);
      })
      .catch((err) => {
        setFetchError(err?.message || String(err));
        setLoading(false);
      });
  }, [name]);

  const checkVaultJobStatus = React.useCallback(async () => {
    if (!patternName) return;

    try {
      setCheckingVaultStatus(true);
      const status = await fetchVaultJobStatus(patternName);
      setVaultJobStatus(status);

      if (status.status === 'running' || status.status === 'pending') {
        setTimeout(() => {
          checkVaultJobStatus();
        }, 5000);
      }
    } catch (err) {
      console.error('Error checking vault job status:', err);
    } finally {
      setCheckingVaultStatus(false);
    }
  }, [patternName]);

  const handleSubmit = async () => {
    setSubmitting(true);
    setSubmitError(null);
    setSuccess(false);
    setVaultJobStatus(null);

    try {
      const yaml = await import('js-yaml');

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
        secrets: secretsList,
      };

      const valuesSecretYaml = yaml.dump(vaultSecretStructure);
      const templateYaml = JSON.stringify(secretTemplate, null, 2);

      const request: VaultInjectionRequest = {
        patternName,
        valuesSecretYaml,
        templateYaml,
      };

      const result = await apiTriggerVaultInjection(request);

      if (result.success) {
        setSuccess(true);
        setTimeout(() => {
          checkVaultJobStatus();
        }, 2000);
      } else {
        setSubmitError(result.message);
      }
    } catch (err) {
      setSubmitError(err?.message || String(err));
    } finally {
      setSubmitting(false);
    }
  };

  const handleFieldChange = (
    secretName: string,
    fieldName: string,
    value: string | File | null,
  ) => {
    setSecretFormData((prev) => ({
      ...prev,
      [secretName]: {
        ...prev[secretName],
        [fieldName]: value,
      },
    }));
  };

  const toggleSection = (sectionName: string) => {
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
    return 'prompt';
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
        <Spinner aria-label={t('Loading secret template')} />
      </PageSection>
    );
  }

  if (fetchError) {
    return (
      <PageSection>
        <Alert variant="danger" title={t('Failed to load secret template')}>
          {fetchError}
        </Alert>
      </PageSection>
    );
  }

  if (!secretTemplate) {
    return (
      <PageSection>
        <Alert variant="info" title={t('No secrets configured')}>
          <p>{t('This pattern does not have a secret template defined.')}</p>
          <Button variant="link" onClick={() => history.push('/patterns')}>
            {t('Back to catalog')}
          </Button>
        </Alert>
      </PageSection>
    );
  }

  return (
    <>
      <Helmet>
        <title>{t('Manage Secrets')}</title>
      </Helmet>
      <PageSection>
        <Title headingLevel="h1">
          {t('Manage Secrets for {{displayName}}', { displayName })}
        </Title>
      </PageSection>
      <PageSection>
        {success && (
          <Alert variant="success" title={t('Secrets submitted successfully')}>
            <p>{t('The vault injection job has been created.')}</p>
          </Alert>
        )}
        {success && vaultJobStatus && (
          <Alert
            style={{ marginTop: 'var(--pf-v6-global--spacer--md)' }}
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
          <Alert variant="danger" title={t('Failed to inject secrets')}>
            {submitError}
          </Alert>
        )}
        <Form
          style={{ marginTop: success ? 'var(--pf-v6-global--spacer--md)' : undefined }}
          onSubmit={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
        >
          <Alert variant="info" title={t('Secret Configuration')} isInline>
            {t('Enter or update the secrets that will be injected into Vault for this pattern. Fields left empty will retain their existing values in Vault.')}
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

          <ActionGroup>
            <Button
              variant="primary"
              type="submit"
              isLoading={submitting}
              isDisabled={submitting}
            >
              {t('Inject Secrets')}
            </Button>
            <Button variant="link" onClick={() => history.push('/patterns')}>
              {t('Back to catalog')}
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
}
