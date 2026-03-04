import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useHistory, useRouteMatch } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Button,
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
import { SecretTemplate, SecretFormData } from '../types';

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

  // Get secret data from navigation state (when returning from secrets page)
  const locationState = history.location.state as
    | {
        secretData?: SecretFormData;
        secretTemplate?: SecretTemplate;
      }
    | undefined;

  const [loading, setLoading] = React.useState(true);
  const [fetchError, setFetchError] = React.useState<string | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  const [submitError, setSubmitError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState(false);

  const [patternName, setPatternName] = React.useState('');
  const [clusterGroupName, setClusterGroupName] = React.useState('hub');
  const [targetRepo, setTargetRepo] = React.useState('');
  const [targetRevision, setTargetRevision] = React.useState('main');

  const [secretTemplate, setSecretTemplate] = React.useState<SecretTemplate | null>(null);
  const [vaultJobStatus, setVaultJobStatus] = React.useState<VaultJobStatus | null>(null);
  const [checkingVaultStatus, setCheckingVaultStatus] = React.useState(false);

  const secretData = locationState?.secretData || null;

  React.useEffect(() => {
    Promise.all([fetchPattern(name), fetchSecretTemplate(name)])
      .then(([patternData, template]) => {
        setPatternName(patternData.name);
        setTargetRepo(patternData.repo_url || '');
        setSecretTemplate(template);

        // Update secret template if returned from secrets page
        if (locationState?.secretTemplate) {
          setSecretTemplate(locationState.secretTemplate);
        }

        setLoading(false);
      })
      .catch((err) => {
        setFetchError(err?.message || String(err));
        setLoading(false);
      });
  }, [name, locationState]);

  const triggerVaultInjection = React.useCallback(async () => {
    if (!patternName || !secretData || !secretTemplate) return;

    try {
      // Convert secretData to YAML format
      const yaml = await import('js-yaml');
      const valuesSecretYaml = yaml.dump(secretData);
      const templateYaml = JSON.stringify(secretTemplate, null, 2);

      const request: VaultInjectionRequest = {
        patternName,
        valuesSecretYaml,
        templateYaml,
      };

      const result = await apiTriggerVaultInjection(request);

      if (result.success) {
        // Start polling for job status
        setTimeout(() => {
          checkVaultJobStatus();
        }, 2000);
      } else {
        setVaultJobStatus({
          status: 'not-found',
          message: result.message,
        });
      }
    } catch (err) {
      console.error('Error triggering vault injection:', err);
      setVaultJobStatus({
        status: 'not-found',
        message: `Failed to trigger vault injection: ${err.message || err}`,
      });
    }
  }, [patternName, secretData, secretTemplate]);

  const checkVaultJobStatus = React.useCallback(async () => {
    if (!patternName) return;

    try {
      setCheckingVaultStatus(true);
      const status = await fetchVaultJobStatus(patternName);
      setVaultJobStatus(status);

      // Continue polling if job is still running or pending
      if (status.status === 'running' || status.status === 'pending') {
        setTimeout(() => {
          checkVaultJobStatus();
        }, 5000); // Poll every 5 seconds
      }
    } catch (err) {
      console.error('Error checking vault job status:', err);
    } finally {
      setCheckingVaultStatus(false);
    }
  }, [patternName]);

  // Check vault job status on component mount if secrets were configured
  React.useEffect(() => {
    if (success && secretData && secretTemplate && patternName) {
      const timer = setTimeout(() => {
        checkVaultJobStatus();
      }, 2000); // Wait 2 seconds after pattern creation
      return () => clearTimeout(timer);
    }
  }, [success, secretData, secretTemplate, patternName, checkVaultJobStatus]);

  const handleSubmit = async () => {
    setSubmitting(true);
    setSubmitError(null);
    try {
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
          clusterGroupName,
          gitSpec: {
            targetRepo,
            targetRevision,
          },
        },
      };

      // Note: Secrets are handled separately after pattern creation

      await k8sCreate({
        model: PatternModel,
        data: patternData,
      });
      setSuccess(true);

      // If secrets were configured, trigger vault injection
      if (secretData && secretTemplate) {
        await triggerVaultInjection();
      }
    } catch (err) {
      setSubmitError(err?.message || String(err));
    } finally {
      setSubmitting(false);
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
        {success && secretData && secretTemplate && vaultJobStatus && (
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
                {t('Job')}: <code>{vaultJobStatus.jobName}</code>
              </p>
            )}
          </Alert>
        )}
        {submitError && (
          <Alert variant="danger" title={t('Failed to create pattern')}>
            {submitError}
          </Alert>
        )}
        {secretData && secretTemplate && (
          <Alert variant="info" title={t('Secrets Configured')} isInline>
            {t('Secret configuration has been provided for this pattern installation.')}
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
            <FormGroup label={t('Cluster Group Name')} isRequired fieldId="pattern-cluster-group">
              <TextInput
                id="pattern-cluster-group"
                isRequired
                value={clusterGroupName}
                onChange={(_event, value) => setClusterGroupName(value)}
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
            <ActionGroup>
              {secretTemplate && (
                <Button
                  variant="secondary"
                  onClick={() => history.push(`/patterns/install/${name}/secrets`)}
                >
                  {secretData ? t('Reconfigure Secrets') : t('Configure Secrets')}
                </Button>
              )}
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
