import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useHistory, useRouteMatch } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  CardBody,
  CardTitle,
  DescriptionList,
  DescriptionListDescription,
  DescriptionListGroup,
  DescriptionListTerm,
  Label,
  PageSection,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { deletePattern, fetchPatternCR, PatternCRStatus } from '../api';

const DELETION_PHASES: Record<string, { label: string; order: number }> = {
  '': { label: 'Starting deletion', order: 0 },
  DeleteSpokeChildApps: { label: 'Deleting spoke child applications', order: 1 },
  DeleteSpoke: { label: 'Deleting spoke app of apps', order: 2 },
  DeleteHubChildApps: { label: 'Deleting hub applications', order: 3 },
  DeleteHub: { label: 'Deleting hub app of apps', order: 4 },
};

export default function UninstallPatternPage() {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const history = useHistory();
  const match = useRouteMatch<{ name: string }>('/patterns/uninstall/:name');
  const name = match?.params?.name;

  const [status, setStatus] = React.useState<PatternCRStatus | null>(null);
  const [deleting, setDeleting] = React.useState(false);
  const [deleted, setDeleted] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [confirmed, setConfirmed] = React.useState(false);

  // Fetch initial status
  React.useEffect(() => {
    if (!name) return;
    fetchPatternCR(name)
      .then((s) => {
        if (!s.exists) {
          setDeleted(true);
        }
        setStatus(s);
      })
      .catch((err) => setError(err?.message || String(err)));
  }, [name]);

  // Poll status after deletion is triggered
  React.useEffect(() => {
    if (!confirmed || deleted || !name) return;

    const interval = setInterval(async () => {
      try {
        const s = await fetchPatternCR(name);
        if (!s.exists) {
          setDeleted(true);
          setDeleting(false);
          clearInterval(interval);
        } else {
          setStatus(s);
        }
      } catch (err) {
        // If the fetch fails with a 404-like error, treat as deleted
        if (err?.message && /404|not found/i.test(err.message)) {
          setDeleted(true);
          setDeleting(false);
          clearInterval(interval);
        }
      }
    }, 3000);

    return () => clearInterval(interval);
  }, [confirmed, deleted, name]);

  const handleDelete = async () => {
    if (!name) return;
    setDeleting(true);
    setError(null);
    try {
      await deletePattern(name);
      // Check immediately if the CR is already gone
      const s = await fetchPatternCR(name);
      if (!s.exists) {
        setDeleted(true);
        setDeleting(false);
      } else {
        setStatus(s);
        setConfirmed(true);
      }
    } catch (err) {
      setError(err?.message || String(err));
      setDeleting(false);
    }
  };

  const getDeletionProgress = (phase: string | undefined): string => {
    const info = DELETION_PHASES[phase || ''];
    return info ? info.label : phase || 'Deleting';
  };

  if (!name) {
    return (
      <PageSection>
        <Alert variant="danger" title={t('No pattern specified')} />
      </PageSection>
    );
  }

  return (
    <>
      <Helmet>
        <title>{t('Uninstall Pattern')}</title>
      </Helmet>
      <PageSection>
        <Title headingLevel="h1">{t('Uninstall Pattern')}: {name}</Title>
      </PageSection>
      <PageSection>
        {deleted && (
          <Alert variant="success" title={t('Pattern successfully removed')}>
            <p>{t('The pattern and all its associated resources have been fully deleted.')}</p>
            <Button variant="link" onClick={() => history.push('/patterns')}>
              {t('Back to catalog')}
            </Button>
          </Alert>
        )}

        {error && (
          <Alert variant="danger" title={t('Error')}>
            {error}
          </Alert>
        )}

        {!deleted && status && status.exists && (
          <Card>
            <CardTitle>{t('Pattern Status')}</CardTitle>
            <CardBody>
              <DescriptionList isHorizontal>
                {status.lastStep && (
                  <DescriptionListGroup>
                    <DescriptionListTerm>{t('Last Step')}</DescriptionListTerm>
                    <DescriptionListDescription>{status.lastStep}</DescriptionListDescription>
                  </DescriptionListGroup>
                )}
                {status.lastError && (
                  <DescriptionListGroup>
                    <DescriptionListTerm>{t('Last Error')}</DescriptionListTerm>
                    <DescriptionListDescription>
                      <Label color="red">{status.lastError}</Label>
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                )}
                {confirmed && (
                  <DescriptionListGroup>
                    <DescriptionListTerm>{t('Deletion Progress')}</DescriptionListTerm>
                    <DescriptionListDescription>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <Spinner size="md" aria-label={t('Deleting')} />
                        <span>{t(getDeletionProgress(status.deletionPhase))}</span>
                      </div>
                    </DescriptionListDescription>
                  </DescriptionListGroup>
                )}
              </DescriptionList>

              {!confirmed && (
                <div style={{ marginTop: '24px' }}>
                  <Alert
                    variant="warning"
                    title={t('This will delete the pattern and all its deployed resources.')}
                    isInline
                  />
                  <div style={{ marginTop: '16px', display: 'flex', gap: '8px' }}>
                    <Button
                      variant="danger"
                      onClick={handleDelete}
                      isLoading={deleting}
                      isDisabled={deleting}
                    >
                      {t('Confirm Uninstall')}
                    </Button>
                    <Button variant="link" onClick={() => history.push('/patterns')}>
                      {t('Cancel')}
                    </Button>
                  </div>
                </div>
              )}
            </CardBody>
          </Card>
        )}

        {!deleted && !status && !error && (
          <Spinner aria-label={t('Loading pattern status')} />
        )}
      </PageSection>
    </>
  );
}
