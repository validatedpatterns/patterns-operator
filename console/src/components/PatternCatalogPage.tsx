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
  Tooltip,
} from '@patternfly/react-core';
import { ExternalLinkAltIcon, InfoCircleIcon } from '@patternfly/react-icons';
import { fetchAllPatterns, fetchInstalledPatterns, fetchCatalogImage } from '../api';
import { Pattern, ClusterRoleRequirements } from '../types';
import './PatternCatalogPage.css';

const CLOUD_LABELS: Record<string, string> = {
  aws: 'AWS',
  gcp: 'GCP',
  azure: 'Azure',
};

function getCloudProviders(pattern: Pattern): string[] {
  if (!pattern.requirements) return [];
  const providers = new Set<string>();
  for (const role of Object.values(pattern.requirements)) {
    for (const nodeType of [role.compute, role.controlPlane]) {
      if (nodeType) {
        Object.keys(nodeType).forEach((p) => providers.add(p));
      }
    }
  }
  return Array.from(providers);
}

function getSizingSummary(role: ClusterRoleRequirements, cloud: string): string | null {
  const control = role.controlPlane?.[cloud];
  const compute = role.compute?.[cloud];
  if (!control && !compute) return null;
  const parts: string[] = [];
  if (control) parts.push(`${control.replicas} control`);
  if (compute) parts.push(`${compute.replicas} compute`);
  return parts.join(' + ');
}

function getSizingTooltip(role: ClusterRoleRequirements, clouds: string[]): string {
  return clouds
    .map((cloud) => {
      const control = role.controlPlane?.[cloud];
      const compute = role.compute?.[cloud];
      const lines: string[] = [];
      if (control) lines.push(`  Control: ${control.replicas}x ${control.type}`);
      if (compute) lines.push(`  Compute: ${compute.replicas}x ${compute.type}`);
      return lines.length ? `${CLOUD_LABELS[cloud] || cloud}:\n${lines.join('\n')}` : null;
    })
    .filter(Boolean)
    .join('\n');
}

const TIER_COLORS: Record<string, 'green' | 'blue' | 'grey'> = {
  maintained: 'green',
  tested: 'blue',
  sandbox: 'grey',
};

const TIER_DESCRIPTIONS: Record<string, string> = {
  maintained: 'Rigorously tested through an automated CI pipeline with continuous validation across OpenShift versions. Highest level of validation and prioritized for ongoing maintenance.',
  tested: 'Undergoes a manual or automated test plan which passes at least once for each new OpenShift Container Platform minor version.',
  sandbox: 'Entry-level patterns that are deployable onto a freshly installed OpenShift cluster without prior modification. May be work-in-progress.',
};

export default function PatternCatalogPage() {
  const { t } = useTranslation('plugin__console-plugin-template');
  const history = useHistory();
  const [patterns, setPatterns] = React.useState<Pattern[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [installedPatterns, setInstalledPatterns] = React.useState<Set<string>>(new Set());
  const [catalogImage, setCatalogImage] = React.useState<string | null>(null);

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

  return (
    <>
      <Helmet>
        <title data-test="pattern-catalog-page-title">{t('Pattern Catalog')}</title>
      </Helmet>
      <PageSection>
        {catalogImage ? (
          <Tooltip content={`${t('Catalog source')}: ${catalogImage}`}>
            <Title headingLevel="h1" style={{ display: 'inline-block' }}>{t('Pattern Catalog')}</Title>
          </Tooltip>
        ) : (
          <Title headingLevel="h1">{t('Pattern Catalog')}</Title>
        )}
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
                  const isInstalled = installedPatterns.has(pattern.name);
                  return (
                  <Card key={pattern.name} className="patterns-operator__card">
                    <CardHeader>
                      <Tooltip content={TIER_DESCRIPTIONS[pattern.tier] || pattern.tier}>
                        <Label color={TIER_COLORS[pattern.tier] || 'grey'}>{pattern.tier}</Label>
                      </Tooltip>
                      {isInstalled && (
                        <Label color="green" className="patterns-operator__installed-label">{t('Installed')}</Label>
                      )}
                    </CardHeader>
                    <Tooltip content={`Org: ${pattern.org}`}>
                      <CardTitle>{pattern.display_name}</CardTitle>
                    </Tooltip>
                    <CardBody>
                      {pattern.description && (
                        <div className="patterns-operator__card-description">{pattern.description}</div>
                      )}
                    </CardBody>
                    <CardBody>
                      {pattern.requirements && (() => {
                        const clouds = getCloudProviders(pattern);
                        const hub = pattern.requirements.hub;
                        const spoke = pattern.requirements.spoke;
                        const defaultCloud = clouds.includes('aws') ? 'aws' : clouds[0];
                        const hubSummary = hub && defaultCloud ? getSizingSummary(hub, defaultCloud) : null;
                        const spokeSummary = spoke && defaultCloud ? getSizingSummary(spoke, defaultCloud) : null;
                        const tooltipParts: string[] = [];
                        if (hub) tooltipParts.push(`Hub:\n${getSizingTooltip(hub, clouds)}`);
                        if (spoke) tooltipParts.push(`Spoke:\n${getSizingTooltip(spoke, clouds)}`);
                        const fullTooltip = tooltipParts.join('\n\n');
                        return (
                          <div className="patterns-operator__requirements">
                            {clouds.length > 0 && (
                              <div className="patterns-operator__cloud-labels">
                                {clouds.map((cloud) => (
                                  <Label key={cloud} color="blue" isCompact>{CLOUD_LABELS[cloud] || cloud}</Label>
                                ))}
                              </div>
                            )}
                            {hubSummary && (
                              <Tooltip content={<pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{fullTooltip}</pre>}>
                                <div className="patterns-operator__sizing-line">
                                  {spoke
                                    ? <>{t('Hub')}: {hubSummary}<br />{t('Spoke')}: {spokeSummary}</>
                                    : `${t('Cluster')}: ${hubSummary}`
                                  }
                                </div>
                              </Tooltip>
                            )}
                            {pattern.external_requirements?.cluster_sizing_note && (
                              <Tooltip content={pattern.external_requirements.cluster_sizing_note.trim()}>
                                <span className="patterns-operator__sizing-note">
                                  <InfoCircleIcon /> {t('Additional requirements')}
                                </span>
                              </Tooltip>
                            )}
                          </div>
                        );
                      })()}
                    </CardBody>
                    <CardFooter className="patterns-operator__card-footer">
                      {isInstalled ? (
                        <>
                          <Button
                            variant="secondary"
                            onClick={() => history.push(`/patterns/secrets/${pattern.catalogKey || pattern.name}`)}
                          >
                            {t('Manage Secrets')}
                          </Button>
                          <Button
                            variant="danger"
                            onClick={() => history.push(`/patterns/uninstall/${pattern.name}`)}
                          >
                            {t('Uninstall')}
                          </Button>
                        </>
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
        )}
      </PageSection>
    </>
  );
}
