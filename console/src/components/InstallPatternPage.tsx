import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useHistory, useParams } from 'react-router-dom';
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
import { fetchPattern } from '../api';
import { Pattern } from '../types';

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
  const { name } = useParams<{ name: string }>();

  const [loading, setLoading] = React.useState(true);
  const [fetchError, setFetchError] = React.useState<string | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  const [submitError, setSubmitError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState(false);

  const [patternName, setPatternName] = React.useState('');
  const [namespace, setNamespace] = React.useState('');
  const [clusterGroupName, setClusterGroupName] = React.useState('hub');
  const [targetRepo, setTargetRepo] = React.useState('');
  const [targetRevision, setTargetRevision] = React.useState('main');

  React.useEffect(() => {
    fetchPattern(name)
      .then((pattern: Pattern) => {
        setPatternName(pattern.name);
        setNamespace(pattern.name);
        setTargetRepo(pattern.repo_url || '');
        setLoading(false);
      })
      .catch((err) => {
        setFetchError(err?.message || String(err));
        setLoading(false);
      });
  }, [name]);

  const handleSubmit = async () => {
    setSubmitting(true);
    setSubmitError(null);
    try {
      await k8sCreate({
        model: PatternModel,
        data: {
          apiVersion: 'gitops.hybrid-cloud-patterns.io/v1alpha1',
          kind: 'Pattern',
          metadata: {
            name: patternName,
            namespace,
          },
          spec: {
            clusterGroupName,
            gitSpec: {
              targetRepo,
              targetRevision,
            },
          },
        },
      });
      setSuccess(true);
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
            <Button variant="link" onClick={() => history.push('/patterns')}>
              {t('Back to catalog')}
            </Button>
          </Alert>
        )}
        {submitError && (
          <Alert variant="danger" title={t('Failed to create pattern')}>
            {submitError}
          </Alert>
        )}
        {!success && (
          <Form onSubmit={(e) => { e.preventDefault(); handleSubmit(); }}>
            <FormGroup label={t('Name')} isRequired fieldId="pattern-name">
              <TextInput
                id="pattern-name"
                isRequired
                value={patternName}
                onChange={(_event, value) => setPatternName(value)}
              />
            </FormGroup>
            <FormGroup label={t('Namespace')} isRequired fieldId="pattern-namespace">
              <TextInput
                id="pattern-namespace"
                isRequired
                value={namespace}
                onChange={(_event, value) => setNamespace(value)}
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
