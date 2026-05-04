import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useHistory, useRouteMatch } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Button,
  Form,
  PageSection,
  Spinner,
  Title,
} from '@patternfly/react-core';
import {
  fetchPattern,
  fetchSecretTemplate,
  triggerVaultInjection as apiTriggerVaultInjection,
} from '../api';
import { useVaultJobPolling } from '../hooks/useVaultJobPolling';
import {
  buildVaultInjectionPayload,
  getMissingFileAndIniFields,
  secretTemplateHasFileOrIniFields,
} from '../vaultSecrets';
import { SecretTemplate, SecretFormData } from '../types';
import { SecretFormExpandableSections } from './SecretForm/SecretFormExpandableSections';
import { VaultInjectionStatusAlert } from './SecretForm/VaultInjectionStatusAlert';
import './SecretForm/SecretForm.css';

export default function ManageSecretsPage() {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
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
  const [secretsValidationAttempted, setSecretsValidationAttempted] = React.useState(false);
  const { vaultJobStatus, setVaultJobStatus, checkingVaultStatus, checkVaultJobStatus } =
    useVaultJobPolling(patternName);

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

  const handleSubmit = async () => {
    if (!secretTemplate) return;

    setSuccess(false);
    setVaultJobStatus(null);

    const missingUploads = getMissingFileAndIniFields(secretTemplate, secretFormData);
    if (missingUploads.length > 0) {
      setSecretsValidationAttempted(true);
      setExpandedSections((prev) => {
        const next = { ...prev };
        missingUploads.forEach(({ secretName }) => {
          next[secretName] = true;
        });
        return next;
      });
      return;
    }
    setSecretsValidationAttempted(false);
    setSubmitError(null);

    setSubmitting(true);
    try {
      const { valuesSecretYaml, fileArtifacts } = buildVaultInjectionPayload(
        secretTemplate,
        secretFormData,
      );

      const request = {
        patternName,
        valuesSecretYaml,
        fileArtifacts,
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

  const handleFieldChange = (secretName: string, fieldName: string, value: string | null) => {
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
        <Title headingLevel="h1">{t('Manage Secrets for {{displayName}}', { displayName })}</Title>
      </PageSection>
      <PageSection>
        {success && (
          <Alert variant="success" title={t('Secrets submitted successfully')}>
            <p>{t('The vault injection job has been created.')}</p>
          </Alert>
        )}
        {success && vaultJobStatus && (
          <VaultInjectionStatusAlert
            style={{ marginTop: '16px' }}
            vaultJobStatus={vaultJobStatus}
            checkingVaultStatus={checkingVaultStatus}
          />
        )}
        {submitError && (
          <Alert variant="danger" title={t('Failed to inject secrets')}>
            {submitError}
          </Alert>
        )}
        <Form
          style={{ marginTop: success ? '16px' : undefined }}
          onSubmit={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
        >
          <Alert variant="info" title={t('Secret Configuration')} isInline>
            {secretTemplateHasFileOrIniFields(secretTemplate)
              ? t(
                  'Enter or update the secrets that will be injected into Vault for this pattern. File and INI fields must be uploaded each time. Other fields may be left empty to keep existing Vault values.',
                )
              : t(
                  'Enter or update the secrets that will be injected into Vault for this pattern. Fields left empty will retain their existing values in Vault.',
                )}
          </Alert>
          <SecretFormExpandableSections
            secrets={secretTemplate.secrets}
            secretFormData={secretFormData}
            expandedSections={expandedSections}
            onToggleSection={toggleSection}
            onFieldChange={handleFieldChange}
            secretsValidationAttempted={secretsValidationAttempted}
          />

          <ActionGroup>
            <Button variant="primary" type="submit" isLoading={submitting} isDisabled={submitting}>
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
