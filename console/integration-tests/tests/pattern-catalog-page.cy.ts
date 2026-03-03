import { checkErrors } from '../support';

const PLUGIN_NAME = 'patterns-operator-console-plugin';
export const isLocalDevEnvironment = Cypress.config('baseUrl').includes('localhost');

// Check if console plugin is installed and enabled (operator-managed)
const checkPluginInstalled = () => {
  cy.visit('/k8s/cluster/operator.openshift.io~v1~Console/cluster/console-plugins');
  cy.get(`[data-test="${PLUGIN_NAME}-status"]`).should('include.text', 'loaded');
};

// For operator-managed deployment, we just need to verify the plugin exists
const verifyOperatorDeployment = () => {
  cy.exec('oc get consoleplugin patterns-operator-console-plugin', {
    failOnNonZeroExit: false,
  }).then((result) => {
    if (result.code !== 0) {
      cy.log('Console plugin not found - operator may not be installed');
    } else {
      cy.log('Console plugin found via operator deployment');
    }
  });
};

describe('Console plugin template test', () => {
  before(() => {
    cy.login();
    cy.get(`[data-test="tour-step-footer-secondary"]`).contains('Skip tour').click();

    if (!isLocalDevEnvironment) {
      console.log('Verifying operator-managed console plugin deployment');
      verifyOperatorDeployment();
    } else {
      console.log('Local development environment - assuming plugin is running via yarn start');
    }
  });

  afterEach(() => {
    checkErrors();
  });

  after(() => {
    // No cleanup needed for operator-managed deployment
    cy.logout();
  });

  it('Verify the example page title', () => {
    cy.get('[data-quickstart-id="qs-nav-home"]').click();
    cy.get('[data-test="nav"]').contains('Plugin Example').click();
    cy.url().should('include', '/example');
    cy.get('[data-test="pattern-catalog-page-title"]').should('contain', 'Pattern Catalog');
  });
});
