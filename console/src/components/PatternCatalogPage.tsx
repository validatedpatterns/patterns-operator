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
  MenuToggle,
  PageSection,
  Select,
  SelectList,
  SelectOption,
  Spinner,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  Tooltip,
} from '@patternfly/react-core';
import { ExternalLinkAltIcon, InfoCircleIcon } from '@patternfly/react-icons';
import { fetchAllPatterns, fetchInstalledPatterns, fetchCatalogImage } from '../api';
import { Pattern, ClusterRoleRequirements } from '../types';
import './PatternCatalogPage.css';

const ALLOWED_TAGS = ['b', 'i', 'em', 'strong', 'a', 'br'];

function sanitizeHTML(html: string): string {
  return html.replace(/<\/?([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>/g, (match, tag) => {
    const lower = tag.toLowerCase();
    if (!ALLOWED_TAGS.includes(lower)) return '';
    if (lower === 'a') {
      const hrefMatch = match.match(/href\s*=\s*"([^"]*)"/);
      if (match.startsWith('</')) return '</a>';
      return hrefMatch
        ? `<a href="${hrefMatch[1]}" target="_blank" rel="noopener noreferrer">`
        : '';
    }
    return match.startsWith('</') ? `</${lower}>` : `<${lower}>`;
  });
}

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

function formatRoleLine(role: ClusterRoleRequirements, cloud: string): string {
  const control = role.controlPlane?.[cloud];
  const compute = role.compute?.[cloud];
  const parts: string[] = [];
  if (control) parts.push(`${control.replicas}× ${control.type} control`);
  if (compute) parts.push(`${compute.replicas}× ${compute.type} compute`);
  return parts.join(', ');
}

function getRequirementsTooltip(hub: ClusterRoleRequirements | undefined, spoke: ClusterRoleRequirements | undefined, clouds: string[]): string {
  return clouds
    .map((cloud) => {
      const lines: string[] = [];
      const hubLine = hub ? formatRoleLine(hub, cloud) : '';
      const spokeLine = spoke ? formatRoleLine(spoke, cloud) : '';
      if (hubLine) lines.push(spoke ? `  Hub: ${hubLine}` : `  ${hubLine}`);
      if (spokeLine) lines.push(`  Spoke: ${spokeLine}`);
      return lines.length ? `${CLOUD_LABELS[cloud] || cloud}\n${lines.join('\n')}` : null;
    })
    .filter(Boolean)
    .join('\n');
}

const TIER_COLORS: Record<string, 'green' | 'blue' | 'orange' | 'grey'> = {
  maintained: 'green',
  tested: 'blue',
  sandbox: 'orange',
};

const TIER_SVG_COLORS: Record<string, { filled: string; outline: string }> = {
  maintained: { filled: '#3e8635', outline: '#3e8635' },
  tested: { filled: '#0066cc', outline: '#0066cc' },
  sandbox: { filled: '#f0ab00', outline: '#f0ab00' },
};

function TierIcon({ tier }: { tier: string }) {
  const colors = TIER_SVG_COLORS[tier] || TIER_SVG_COLORS.sandbox;
  // 3 horizontal bars: bottom=bar1, middle=bar2, top=bar3
  // maintained: all 3 filled; tested: 2 filled + 1 outline; sandbox: 1 filled + 2 outline
  const filledCount = tier === 'maintained' ? 3 : tier === 'tested' ? 2 : 1;
  return (
    <svg width="16" height="16" viewBox="0 0 48 48" style={{ verticalAlign: 'middle', marginRight: '4px' }}>
      {[0, 1, 2].map((i) => {
        const y = 34 - i * 14;
        const filled = i < filledCount;
        return (
          <rect
            key={i}
            x="4" y={y} width="40" height="10" rx="5"
            fill={filled ? colors.filled : 'none'}
            stroke={colors.outline}
            strokeWidth={filled ? 0 : 3}
          />
        );
      })}
    </svg>
  );
}

const TIER_DESCRIPTIONS: Record<string, string> = {
  maintained: 'Rigorously tested through an automated CI pipeline with continuous validation across OpenShift versions. Highest level of validation and prioritized for ongoing maintenance.',
  tested: 'Undergoes a manual or automated test plan which passes at least once for each new OpenShift Container Platform minor version.',
  sandbox: 'Entry-level patterns that are deployable onto a freshly installed OpenShift cluster without prior modification. May be work-in-progress.',
};

export default function PatternCatalogPage() {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const history = useHistory();
  const [patterns, setPatterns] = React.useState<Pattern[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [installedPatterns, setInstalledPatterns] = React.useState<Set<string>>(new Set());
  const [catalogImage, setCatalogImage] = React.useState<string | null>(null);
  const [catalogDescription, setCatalogDescription] = React.useState<string | undefined>();
  const [selectedTiers, setSelectedTiers] = React.useState<Set<string>>(new Set(['maintained']));
  const [tierSelectOpen, setTierSelectOpen] = React.useState(false);

  const loadData = React.useCallback(() => {
    setLoading(true);
    Promise.all([fetchAllPatterns(), fetchInstalledPatterns(), fetchCatalogImage()])
      .then(([catalogData, installed, image]) => {
        setPatterns(catalogData.patterns);
        setCatalogDescription(catalogData.catalogDescription);
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

  const filteredPatterns = React.useMemo(
    () => selectedTiers.size === 0 ? patterns : patterns.filter((p) => selectedTiers.has(p.tier)),
    [patterns, selectedTiers],
  );

  const onTierSelect = (_event: React.MouseEvent | undefined, value: string | number | undefined) => {
    setSelectedTiers((prev) => {
      const next = new Set(prev);
      if (next.has(value as string)) {
        next.delete(value as string);
      } else {
        next.add(value as string);
      }
      return next;
    });
  };

  const tierToggleLabel = selectedTiers.size === 0
    ? t('Tier')
    : Array.from(selectedTiers).map((tier) => tier.charAt(0).toUpperCase() + tier.slice(1)).join(', ');

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
      {catalogDescription && (
        <PageSection>
          <p dangerouslySetInnerHTML={{ __html: sanitizeHTML(catalogDescription) }} />
        </PageSection>
      )}
      <PageSection>
        {loading && <Spinner aria-label={t('Loading patterns')} />}
        {error && (
          <Alert variant="danger" title={t('Failed to load pattern catalog')}>
            {error}
          </Alert>
        )}
        {!loading && !error && (
          <>
          <Toolbar>
            <ToolbarContent>
              <ToolbarItem>
                <Select
                  role="menu"
                  id="tier-filter"
                  isOpen={tierSelectOpen}
                  selected={Array.from(selectedTiers)}
                  onSelect={onTierSelect}
                  onOpenChange={setTierSelectOpen}
                  toggle={(toggleRef) => (
                    <MenuToggle
                      ref={toggleRef}
                      onClick={() => setTierSelectOpen((prev) => !prev)}
                      isExpanded={tierSelectOpen}
                    >
                      {tierToggleLabel}
                    </MenuToggle>
                  )}
                >
                  <SelectList>
                    {['maintained', 'tested', 'sandbox'].map((tier) => (
                      <SelectOption
                        key={tier}
                        value={tier}
                        hasCheckbox
                        isSelected={selectedTiers.has(tier)}
                      >
                        {tier.charAt(0).toUpperCase() + tier.slice(1)}
                      </SelectOption>
                    ))}
                  </SelectList>
                </Select>
              </ToolbarItem>
            </ToolbarContent>
          </Toolbar>
          <Gallery hasGutter minWidths={{ default: '300px' }}>
                {filteredPatterns.map((pattern) => {
                  const isInstalled = installedPatterns.has(pattern.name);
                  return (
                  <Card key={pattern.name} className="patterns-operator__card">
                    <CardHeader>
                      <Tooltip content={TIER_DESCRIPTIONS[pattern.tier] || pattern.tier}>
                        <Label color={TIER_COLORS[pattern.tier] || 'grey'} icon={<TierIcon tier={pattern.tier} />}>{pattern.tier}</Label>
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
                        const fullTooltip = getRequirementsTooltip(hub, spoke, clouds);
                        return (
                          <div className="patterns-operator__requirements">
                            <Tooltip content={t('This is the sizing that has been tested. The pattern is expected to work on any similarly-sized architecture.')}>
                              <div className="patterns-operator__requirements-heading">{t('Tested Requirements:')}</div>
                            </Tooltip>
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
                      {(pattern.docs_url || pattern.repo_url) && (
                        <div className="patterns-operator__card-links">
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
                        </div>
                      )}
                      <div className="patterns-operator__card-actions">
                        {isInstalled && (
                          <Button
                            variant="secondary"
                            onClick={() =>
                              history.push(`/patterns/secrets/${pattern.catalogKey || pattern.name}`)
                            }
                          >
                            {t('Manage Secrets')}
                          </Button>
                        )}
                        {isInstalled && (
                          <Button
                            variant="danger"
                            onClick={() =>
                            history.push(`/patterns/uninstall/${pattern.name}`)
                            }
                          >
                            {t('Uninstall')}
                          </Button>
                        )}
                        {!isInstalled && (
                          <Button
                            variant="primary"
                            onClick={() =>
                              history.push(`/patterns/install/${pattern.catalogKey || pattern.name}`)
                            }
                          >
                            {t('Install')}
                          </Button>
                        )}
                      </div>
                    </CardFooter>
                  </Card>
                  );
                })}
          </Gallery>
          </>
        )}
      </PageSection>
    </>
  );
}
