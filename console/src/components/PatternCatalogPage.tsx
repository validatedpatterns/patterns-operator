import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useHistory } from 'react-router-dom';
import {
  Alert,
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  CardTitle,
  Gallery,
  Label,
  PageSection,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { ExternalLinkAltIcon } from '@patternfly/react-icons';
import { fetchAllPatterns, fetchInstalledPatterns, fetchCatalogImage, deletePattern } from '../api';
import { Pattern } from '../types';
import './PatternCatalogPage.css';

const TIER_COLORS: Record<string, 'green' | 'blue' | 'grey'> = {
  maintained: 'green',
  tested: 'blue',
  sandbox: 'grey',
};

export default function PatternCatalogPage() {
  const { t } = useTranslation('plugin__console-plugin-template');
  const history = useHistory();
  const [patterns, setPatterns] = React.useState<Pattern[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [installedPatterns, setInstalledPatterns] = React.useState<Set<string>>(new Set());
  const [catalogImage, setCatalogImage] = React.useState<string | null>(null);
  const [uninstalling, setUninstalling] = React.useState<string | null>(null);
  const [uninstallError, setUninstallError] = React.useState<string | null>(null);

  const loadData = React.useCallback(() => {
    setLoading(true);
    Promise.all([fetchAllPatterns(), fetchInstalledPatterns(), fetchCatalogImage()])
      .then(([data, installed, image]) => {
        setPatterns(data);
        setInstalledPatterns(new Set(installed));
        setCatalogImage(image);
        setLoading(false);
      })
      .catch((err) => {
        setError(err?.message || String(err));
        setLoading(false);
      });
  }, []);

  React.useEffect(() => {
    loadData();
  }, [loadData]);

  const handleUninstall = async (patternName: string) => {
    setUninstalling(patternName);
    setUninstallError(null);
    try {
      await deletePattern(patternName);
      setInstalledPatterns((prev) => {
        const next = new Set(prev);
        next.delete(patternName);
        return next;
      });
    } catch (err) {
      setUninstallError(err?.message || String(err));
    } finally {
      setUninstalling(null);
    }
  };

  return (
    <>
      <Helmet>
        <title data-test="pattern-catalog-page-title">{t('Pattern Catalog')}</title>
      </Helmet>
      <PageSection>
        <Title headingLevel="h1">{t('Pattern Catalog')}</Title>
        {catalogImage && (
          <div className="patterns-operator__catalog-source">
            {t('Catalog source')}: <code>{catalogImage}</code>
          </div>
        )}
      </PageSection>
      <PageSection>
        {loading && <Spinner aria-label={t('Loading patterns')} />}
        {error && (
          <Alert variant="danger" title={t('Failed to load pattern catalog')}>
            {error}
          </Alert>
        )}
        {uninstallError && (
          <Alert variant="danger" title={t('Failed to uninstall pattern')} isInline>
            {uninstallError}
          </Alert>
        )}
        {!loading && !error && (
          <div className="patterns-operator__catalog-layout">
            <div className="patterns-operator__catalog-main">
              <Gallery hasGutter minWidths={{ default: '300px' }}>
                {patterns.map((pattern) => {
                  const isInstalled = installedPatterns.has(pattern.name);
                  const isUninstalling = uninstalling === pattern.name;
                  return (
                  <Card key={pattern.name} className="patterns-operator__card">
                    <CardHeader>
                      <Label color={TIER_COLORS[pattern.tier] || 'grey'}>{pattern.tier}</Label>
                      {isInstalled && (
                        <Label color="green" className="patterns-operator__installed-label">{t('Installed')}</Label>
                      )}
                    </CardHeader>
                    <CardTitle>{pattern.display_name}</CardTitle>
                    <CardBody />
                    <CardFooter className="patterns-operator__card-footer">
                      {isInstalled ? (
                        <Button
                          variant="danger"
                          onClick={() => handleUninstall(pattern.name)}
                          isLoading={isUninstalling}
                          isDisabled={isUninstalling}
                        >
                          {t('Uninstall')}
                        </Button>
                      ) : (
                        <Button
                          variant="primary"
                          onClick={() =>
                            history.push(`/patterns/install/${pattern.catalogKey || pattern.name}`)
                          }
                        >
                          {t('Install')}
                        </Button>
                      )}
                      {pattern.docs_url && (
                        <Button
                          variant="link"
                          component="a"
                          href={pattern.docs_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          icon={<ExternalLinkAltIcon />}
                          iconPosition="end"
                        >
                          {t('Docs')}
                        </Button>
                      )}
                      {pattern.repo_url && (
                        <Button
                          variant="link"
                          component="a"
                          href={pattern.repo_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          icon={<ExternalLinkAltIcon />}
                          iconPosition="end"
                        >
                          {t('Repo')}
                        </Button>
                      )}
                    </CardFooter>
                  </Card>
                  );
                })}
              </Gallery>
            </div>
            <div className="patterns-operator__catalog-sidebar">
              <Card>
                <CardTitle>{t('Pattern Tiers')}</CardTitle>
                <CardBody>
                  <div className="patterns-operator__tier-item">
                    <Label color="green">{t('maintained')}</Label>
                    <p>{t('Rigorously tested through an automated CI pipeline with continuous validation across OpenShift versions. Highest level of validation and prioritized for ongoing maintenance.')}</p>
                  </div>
                  <div className="patterns-operator__tier-item">
                    <Label color="blue">{t('tested')}</Label>
                    <p>{t('Undergoes a manual or automated test plan which passes at least once for each new OpenShift Container Platform minor version.')}</p>
                  </div>
                  <div className="patterns-operator__tier-item">
                    <Label color="grey">{t('sandbox')}</Label>
                    <p>{t('Entry-level patterns that are deployable onto a freshly installed OpenShift cluster without prior modification. May be work-in-progress.')}</p>
                  </div>
                </CardBody>
              </Card>
            </div>
          </div>
        )}
      </PageSection>
    </>
  );
}
