const dismissTour = () => {
  cy.get('body').then(($body) => {
    if ($body.find('[data-test="tour-step-footer-secondary"]').length > 0) {
      cy.get('[data-test="tour-step-footer-secondary"]').contains('Skip tour').click();
    }
  });
};

const navigateToInstallPage = () => {
  cy.visit('/patterns');
  cy.get('.patterns-operator__card', { timeout: 60000 }).should('exist');
  cy.get('.patterns-operator__card-actions')
    .contains('button:not(:disabled)', 'Install')
    .first()
    .click();
  cy.contains('h1', 'Install Pattern', { timeout: 60000 }).should('be.visible');
};

describe('Install Pattern Page', () => {
  before(function () {
    cy.login();
    dismissTour();

    // Check if Install is available; skip the entire suite if not
    cy.visit('/patterns');
    cy.get('.patterns-operator__card', { timeout: 60000 }).should('exist');
    cy.get('body').then(($body) => {
      const installBtn = $body.find(
        '.patterns-operator__card-actions button:not(:disabled):contains("Install")',
      );
      if (installBtn.length === 0) {
        this.skip();
      }
    });
  });

  after(() => {
    cy.logout();
  });

  it('displays the Install Pattern title', () => {
    navigateToInstallPage();
    cy.contains('h1', 'Install Pattern').should('be.visible');
  });

  it('form fields are pre-populated from catalog data', () => {
    navigateToInstallPage();
    cy.get('#pattern-name').invoke('val').should('not.be.empty');
    cy.get('#pattern-target-repo').invoke('val').should('not.be.empty');
    cy.get('#pattern-target-revision').should('have.value', 'main');
  });

  it('target repo is disabled by default', () => {
    navigateToInstallPage();
    cy.get('#pattern-target-repo').should('be.disabled');
  });

  it('use-own-fork checkbox enables the target repo field', () => {
    navigateToInstallPage();
    cy.get('#pattern-target-repo').should('be.disabled');
    cy.get('#use-own-fork').check();
    cy.get('#pattern-target-repo').should('not.be.disabled');
    cy.get('#use-own-fork').uncheck();
    cy.get('#pattern-target-repo').should('be.disabled');
  });

  it('has Install and Cancel buttons', () => {
    navigateToInstallPage();
    cy.contains('button', 'Install').scrollIntoView().should('be.visible');
    cy.contains('button', 'Cancel').scrollIntoView().should('be.visible');
  });

  it('Cancel button returns to the catalog', () => {
    navigateToInstallPage();
    cy.contains('button', 'Cancel').click();
    cy.url().should('include', '/patterns');
    cy.url().should('not.include', '/install');
    cy.contains('h1', 'Pattern Catalog', { timeout: 60000 }).should('be.visible');
  });
});
