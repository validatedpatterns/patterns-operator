const PLUGIN_NAME = 'patterns-operator-console-plugin';
const isLocalDevEnvironment = Cypress.config('baseUrl').includes('localhost');

const dismissTour = () => {
  cy.get('body').then(($body) => {
    if ($body.find('[data-test="tour-step-footer-secondary"]').length > 0) {
      cy.get('[data-test="tour-step-footer-secondary"]').contains('Skip tour').click();
    }
  });
};

const visitCatalog = () => {
  cy.visit('/patterns');
  cy.get('.patterns-operator__card', { timeout: 60000 }).should('exist');
};

describe('Pattern Catalog Page', () => {
  before(() => {
    cy.login();
    dismissTour();
  });

  after(() => {
    cy.logout();
  });

  it('displays the page title', () => {
    cy.visit('/patterns');
    cy.contains('h1', 'Pattern Catalog', { timeout: 60000 }).should('be.visible');
  });

  it('loads and displays pattern cards', () => {
    visitCatalog();
    cy.get('.patterns-operator__card').should('have.length.greaterThan', 0);
  });

  it('pattern cards show tier labels', () => {
    visitCatalog();
    cy.get('.patterns-operator__card').first().within(() => {
      cy.get('.pf-v6-c-label').should('exist');
    });
  });

  it('at least one pattern card displays a description', () => {
    visitCatalog();
    cy.get('.patterns-operator__card-description')
      .should('have.length.greaterThan', 0)
      .first()
      .invoke('text')
      .should('not.be.empty');
  });

  it('pattern cards have external Docs and Repo links', () => {
    visitCatalog();
    cy.get('.patterns-operator__card-links').first().within(() => {
      cy.contains('a', 'Docs')
        .should('have.attr', 'target', '_blank')
        .and('have.attr', 'href');
      cy.contains('a', 'Repo')
        .should('have.attr', 'target', '_blank')
        .and('have.attr', 'href');
    });
  });

  it('pattern cards have action buttons', () => {
    visitCatalog();
    cy.get('.patterns-operator__card-actions').first().within(() => {
      cy.get('button').should('have.length.greaterThan', 0);
    });
  });

  it('tier filter dropdown shows all tier options', () => {
    visitCatalog();
    // Default filter shows "Maintained"; click the toggle button
    cy.contains('button', 'Maintained').click();
    // Options are capitalized ("Tested", "Sandbox") and unique to the dropdown
    cy.contains('Tested').should('be.visible');
    cy.contains('Sandbox').should('be.visible');
    // Close dropdown by clicking the toggle again
    cy.contains('button', 'Maintained').click();
  });

  it('selecting all tiers shows at least as many cards as maintained only', () => {
    visitCatalog();
    cy.get('.patterns-operator__card').its('length').then((maintainedCount) => {
      // Open filter and add Tested
      cy.contains('button', 'Maintained').click();
      cy.contains('Tested').click();
      // Dropdown may close after selection; re-open to add Sandbox
      cy.contains('button', /Maintained/).click();
      cy.contains('Sandbox').click();
      // Close dropdown
      cy.get('body').click(0, 0);
      // With more tiers selected, card count should be >= maintained only
      cy.get('.patterns-operator__card').should('have.length.gte', maintainedCount);
    });
  });

  it('clicking Install navigates to the install page', () => {
    visitCatalog();
    cy.get('body').then(($body) => {
      const installBtn = $body.find('.patterns-operator__card-actions button:not(:disabled):contains("Install")');
      if (installBtn.length === 0) {
        cy.log('No Install button available (a pattern may already be installed)');
        return;
      }
      cy.get('.patterns-operator__card-actions')
        .contains('button:not(:disabled)', 'Install')
        .first()
        .click();
      cy.url().should('include', '/patterns/install/');
      cy.contains('h1', 'Install Pattern', { timeout: 60000 }).should('be.visible');
    });
  });

  it('Patterns section is visible in the sidebar navigation', () => {
    cy.visit('/patterns');
    cy.contains('h1', 'Pattern Catalog', { timeout: 60000 }).should('be.visible');
    cy.get('nav').contains('Patterns').should('be.visible');
  });
});
