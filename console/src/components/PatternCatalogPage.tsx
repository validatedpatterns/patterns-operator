import * as React from 'react';
import Helmet from 'react-helmet';
import { useTranslation } from 'react-i18next';
import { useNavigateCompat } from '../hooks/useNavigateCompat';

import {
  Alert,
  Gallery,
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
import { fetchAllPatterns, fetchInstalledPatterns, fetchCatalogImage } from '../api';
import { Pattern } from '../types';
import './PatternCatalogPage.css';
import PatternCard from './PatternCard';
import useLocalStorage from '../hooks/useLocalStorage';
import sanitizeHtml from 'sanitize-html';

function sanitize(html: string): string {
  return sanitizeHtml(html, {
    allowedTags: ['b', 'i', 'em', 'strong', 'a', 'br'],
    allowedAttributes: {
      a: ['href'],
    },
    transformTags: {
      a: sanitizeHtml.simpleTransform('a', {
        target: '_blank',
        rel: 'noopener noreferrer',
      }),
    },
  });
}

const KNOWN_TIER_ORDER = ['maintained', 'tested', 'sandbox'];

export default function PatternCatalogPage() {
  const { t } = useTranslation('plugin__patterns-operator-console-plugin');
  const navigate = useNavigateCompat();
  const [patterns, setPatterns] = React.useState<Pattern[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [installedPatterns, setInstalledPatterns] = React.useState<Set<string>>(new Set());
  const [catalogImage, setCatalogImage] = React.useState<string | null>(null);
  const [catalogDescription, setCatalogDescription] = React.useState<string | undefined>();
  const [catalogLogo, setCatalogLogo] = React.useState<string | undefined>();
  const [storedTiers, setStoredTiers] = useLocalStorage<string[] | null>(
    'patterns-operator__catalog-selected-tiers',
    null,
  );
  const [tierSelectOpen, setTierSelectOpen] = React.useState(false);

  const loadData = React.useCallback(() => {
    setLoading(true);
    Promise.all([fetchAllPatterns(), fetchInstalledPatterns(), fetchCatalogImage()])
      .then(([catalogData, installed, image]) => {
        setPatterns(catalogData.patterns);
        setCatalogDescription(catalogData.catalogDescription);
        setCatalogLogo(catalogData.catalogLogo);
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

  const availableTiers = React.useMemo(() => {
    const tierSet = new Set(patterns.map((p) => p.tier));
    return Array.from(tierSet).sort((a, b) => {
      const ai = KNOWN_TIER_ORDER.indexOf(a);
      const bi = KNOWN_TIER_ORDER.indexOf(b);
      if (ai !== -1 && bi !== -1) return ai - bi;
      if (ai !== -1) return -1;
      if (bi !== -1) return 1;
      return a.localeCompare(b, undefined, { sensitivity: 'base' });
    });
  }, [patterns]);

  const defaultTiers = React.useMemo(
    () => (availableTiers.includes('maintained') ? ['maintained'] : availableTiers),
    [availableTiers],
  );
  const selectedTiers = storedTiers ?? defaultTiers;

  const filteredPatterns = React.useMemo(
    () =>
      selectedTiers.length === 0
        ? patterns
        : patterns.filter((p) => selectedTiers.includes(p.tier)),
    [patterns, selectedTiers],
  );

  const onTierSelect = (
    _event: React.MouseEvent | undefined,
    value: string | number | undefined,
  ) => {
    const tier = value as string;
    setStoredTiers(
      selectedTiers.includes(tier)
        ? selectedTiers.filter((t) => t !== tier)
        : [...selectedTiers, tier],
    );
  };

  const tierToggleLabel =
    selectedTiers.length === 0
      ? t('Tier')
      : selectedTiers.map((tier) => tier.charAt(0).toUpperCase() + tier.slice(1)).join(', ');

  return (
    <>
      <Helmet>
        <title data-test="pattern-catalog-page-title">{t('Pattern Catalog')}</title>
      </Helmet>
      <PageSection>
        <div className="patterns-operator__catalog-header">
          {catalogLogo && (
            <img
              src={catalogLogo}
              alt={t('Catalog logo')}
              className="patterns-operator__catalog-logo"
            />
          )}
          {catalogImage ? (
            <Tooltip content={`${t('Catalog source')}: ${catalogImage}`}>
              <Title
                headingLevel="h1"
                data-test="pattern-catalog-page-title"
                style={{ display: 'inline-block' }}
              >
                {t('Pattern Catalog')}
              </Title>
            </Tooltip>
          ) : (
            <Title headingLevel="h1" data-test="pattern-catalog-page-title">
              {t('Pattern Catalog')}
            </Title>
          )}
        </div>
      </PageSection>
      {catalogDescription && (
        <PageSection>
          <p dangerouslySetInnerHTML={{ __html: sanitize(catalogDescription) }} />
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
                    selected={selectedTiers}
                    onSelect={onTierSelect}
                    onOpenChange={setTierSelectOpen}
                    toggle={(toggleRef) => (
                      <MenuToggle
                        ref={toggleRef}
                        id="tier-filter-toggle"
                        onClick={() => setTierSelectOpen((prev) => !prev)}
                        isExpanded={tierSelectOpen}
                      >
                        {tierToggleLabel}
                      </MenuToggle>
                    )}
                  >
                    <SelectList>
                      {availableTiers.map((tier) => (
                        <SelectOption
                          key={tier}
                          value={tier}
                          hasCheckbox
                          isSelected={selectedTiers.includes(tier)}
                        >
                          {tier.charAt(0).toUpperCase() + tier.slice(1)}
                        </SelectOption>
                      ))}
                    </SelectList>
                  </Select>
                </ToolbarItem>
              </ToolbarContent>
            </Toolbar>
            <Gallery hasGutter minWidths={{ default: '320px' }}>
              {filteredPatterns.map((pattern) => {
                const isInstalled = installedPatterns.has(pattern.name);
                const hasAnyInstalled = installedPatterns.size > 0;
                const isDisabled = hasAnyInstalled && !isInstalled;
                return (
                  <PatternCard
                    key={pattern.name}
                    pattern={pattern}
                    isInstalled={isInstalled}
                    isDisabled={isDisabled}
                    navigate={navigate}
                  />
                );
              })}
            </Gallery>
          </>
        )}
      </PageSection>
    </>
  );
}
