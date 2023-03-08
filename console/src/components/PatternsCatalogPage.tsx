import * as React from 'react';
import Helmet from 'react-helmet';
import { Page, PageSection, TextContent, Text, Title } from '@patternfly/react-core';
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
        <PageSection variant="light">
          <TextContent>
            <Text component="p">
              <span className="console-plugin-template__nice">Hybrid Cloud
              Patterns</span> are an evolution of how you deploy applications in
              a hybrid cloud. With a pattern, you can automatically deploy a
              full application stack through a GitOps-based framework. With this
              framework, you can create business-centric solutions while
              maintaining a level of Continuous Integration (CI) over your
              application.
            </Text>
          </TextContent>
        </PageSection>
        <PatternsCatalog />
      </Page>
    </>
  );
}
