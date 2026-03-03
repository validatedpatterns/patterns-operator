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
import { fetchAllPatterns } from '../api';
import { Pattern } from '../types';
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
            {patterns.map((pattern) => (
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
                </CardBody>
                <CardFooter className="patterns-operator__card-footer">
                  <Button
                    variant="primary"
                    onClick={() =>
                      history.push(`/patterns/install/${pattern.catalogKey || pattern.name}`)
                    }
                  >
                    {t('Install')}
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
            ))}
          </Gallery>
        )}
      </PageSection>
    </>
  );
}
