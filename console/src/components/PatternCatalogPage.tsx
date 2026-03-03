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
  Popover,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { ExternalLinkAltIcon, InfoCircleIcon } from '@patternfly/react-icons';
import { fetchAllPatterns } from '../api';
import { Pattern } from '../types';
import { useClusterInfo } from '../cluster-api';
import {
  checkPatternCompatibility,
  getCompatibilityColor,
  getCompatibilityLabel,
  getInstallButtonText,
  getInstallButtonVariant,
} from '../compatibility';
import CompatibilityDetails from './CompatibilityDetails';
import './PatternCatalogPage.css';

const TIER_COLORS: Record<string, 'green' | 'blue' | 'grey'> = {
  maintained: 'green',
  tested: 'blue',
  sandbox: 'grey',
};

function getCloudProviders(pattern: Pattern): string[] {
  const compute = pattern.requirements?.hub?.compute;
  if (!compute) return [];
  return Object.keys(compute).map((k) => k.toUpperCase());
}

export default function PatternCatalogPage() {
  const { t } = useTranslation('plugin__console-plugin-template');
  const history = useHistory();
  const [patterns, setPatterns] = React.useState<Pattern[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  // Fetch cluster information for compatibility checking
  const [clusterInfo, clusterLoading, clusterError] = useClusterInfo();

  React.useEffect(() => {
    fetchAllPatterns()
      .then((data) => {
        setPatterns(data);
        setLoading(false);
      })
      .catch((err) => {
        setError(err?.message || String(err));
        setLoading(false);
      });
  }, []);

  return (
    <>
      <Helmet>
        <title data-test="pattern-catalog-page-title">{t('Pattern Catalog')}</title>
      </Helmet>
      <PageSection>
        <Title headingLevel="h1">{t('Pattern Catalog')}</Title>
      </PageSection>
      <PageSection>
        {loading && <Spinner aria-label={t('Loading patterns')} />}
        {error && (
          <Alert variant="danger" title={t('Failed to load pattern catalog')}>
            {error}
          </Alert>
        )}
        {!loading && !error && (
          <Gallery hasGutter minWidths={{ default: '300px' }}>
            {patterns.map((pattern) => {
              // Calculate compatibility for this pattern
              const compatibilityResult = clusterInfo
                ? checkPatternCompatibility(pattern, clusterInfo)
                : null;

              return (
                <Card key={pattern.name} className="patterns-operator__card">
                  <CardHeader>
                    <Label color={TIER_COLORS[pattern.tier] || 'grey'}>{pattern.tier}</Label>
                  </CardHeader>
                  <CardTitle>{pattern.display_name}</CardTitle>
                  <CardBody>
                    <div className="patterns-operator__card-field">
                      <strong>{t('Organization')}:</strong> {pattern.org}
                    </div>
                    {getCloudProviders(pattern).length > 0 && (
                      <div className="patterns-operator__card-field">
                        <strong>{t('Cloud Providers')}:</strong>{' '}
                        {getCloudProviders(pattern).join(', ')}
                      </div>
                    )}
                    <div className="patterns-operator__card-field">
                      <strong>{t('Owners')}:</strong> {pattern.owners?.join(', ')}
                    </div>
                    {/* Compatibility status display */}
                    {clusterInfo && compatibilityResult && (
                      <div className="patterns-operator__card-field">
                        <strong>{t('Compatibility')}:</strong>{' '}
                        <div className="patterns-operator__compatibility-status">
                          <Label color={getCompatibilityColor(compatibilityResult.status)}>
                            {getCompatibilityLabel(compatibilityResult.status)}
                          </Label>
                          <Popover
                            headerContent={t('Compatibility Details')}
                            bodyContent={
                              <CompatibilityDetails
                                result={compatibilityResult}
                                clusterInfo={clusterInfo}
                                pattern={pattern}
                              />
                            }
                            position="right"
                            maxWidth="600px"
                          >
                            <InfoCircleIcon className="patterns-operator__info-icon" />
                          </Popover>
                        </div>
                      </div>
                    )}
                    {/* Show loading or error state for cluster compatibility */}
                    {clusterLoading && (
                      <div className="patterns-operator__card-field">
                        <strong>{t('Compatibility')}:</strong>{' '}
                        <span>{t('Checking...')}</span>
                      </div>
                    )}
                    {clusterError && (
                      <div className="patterns-operator__card-field">
                        <strong>{t('Compatibility')}:</strong>{' '}
                        <div className="patterns-operator__compatibility-status">
                          <Label color="orange">{t('Check Failed')}</Label>
                          <Popover
                            headerContent={t('Compatibility Check Error')}
                            bodyContent={
                              <div>
                                <p>{t('Unable to determine cluster compatibility:')}</p>
                                <code style={{ fontSize: '0.9em', wordBreak: 'break-word' }}>
                                  {clusterError}
                                </code>
                                <p style={{ marginTop: '8px' }}>
                                  {t('This may be due to insufficient permissions or cluster connectivity issues.')}
                                </p>
                              </div>
                            }
                            position="right"
                            maxWidth="500px"
                          >
                            <InfoCircleIcon className="patterns-operator__info-icon" />
                          </Popover>
                        </div>
                      </div>
                    )}
                  </CardBody>
                  <CardFooter className="patterns-operator__card-footer">
                    <Button
                      variant={compatibilityResult ? getInstallButtonVariant(compatibilityResult.status) : 'primary'}
                      onClick={() =>
                        history.push(`/patterns/install/${pattern.catalogKey || pattern.name}`)
                      }
                    >
                      {compatibilityResult ? getInstallButtonText(compatibilityResult.status) : t('Install')}
                    </Button>
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
        )}
      </PageSection>
    </>
  );
}
