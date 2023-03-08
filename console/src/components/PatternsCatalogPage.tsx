import * as React from 'react';
import Helmet from 'react-helmet';
import { Page, PageSection, Title } from '@patternfly/react-core';
import PatternsCatalog from './PatternsCatalog';
import '../main.css';

export default function PatternsCatalogPage() {
  // https://www.patternfly.org/v4/extensions/catalog-view/catalog-tile
  return (
    <>
      <Helmet>
        <title data-test="example-page-title">Pattern Catalog</title>
      </Helmet>
      <Page>
        <PageSection variant="light">
          <Title headingLevel="h1">Pattern Catalog</Title>
        </PageSection>
        <PatternsCatalog />
      </Page>
    </>
  );
}
