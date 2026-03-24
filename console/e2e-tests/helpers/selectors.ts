export const selectors = {
  // PatternCatalogPage
  catalogPageTitle: 'h1:has-text("Pattern Catalog")',
  tierFilterToggle: '#content-scrollable .pf-v6-c-menu-toggle',
  patternCard: '.patterns-operator__card',
  installedLabel: '.patterns-operator__installed-label',
  installButton: 'button:has-text("Install")',
  uninstallButton: 'button:has-text("Uninstall")',
  manageSecretsButton: 'button:has-text("Manage Secrets")',
  docsLink: 'a:has-text("Docs")',
  repoLink: 'a:has-text("Repo")',
  cloudLabel: '.patterns-operator__cloud-labels .pf-v6-c-label',
  spinner: '.pf-v6-c-spinner',
  alertDanger: '.pf-v6-c-alert.pf-m-danger',
  catalogDescription: '#content-scrollable p',

  // InstallPatternPage
  installPageTitle: 'h1:has-text("Install Pattern")',
  patternNameInput: '#pattern-name',
  targetRepoInput: '#pattern-target-repo',
  targetRevisionInput: '#pattern-target-revision',
  submitInstallButton: 'button[type="submit"]:has-text("Install")',
  cancelButton: 'button:has-text("Cancel")',
  secretSection: '.patterns-operator__secret-section',
  secretField: '.patterns-operator__secret-field',

  // UninstallPatternPage
  uninstallPageTitle: 'h1:has-text("Uninstall Pattern")',
  confirmUninstallButton: 'button:has-text("Confirm Uninstall")',
  uninstallWarning: '.pf-v6-c-alert.pf-m-warning',
  patternStatusCard: '.pf-v6-c-card:has-text("Pattern Status")',

  // ManageSecretsPage
  manageSecretsPageTitle: 'h1:has-text("Manage Secrets")',
  injectSecretsButton: 'button:has-text("Inject Secrets")',
  backToCatalogLink: 'button:has-text("Back to catalog")',
  secretForm: '.patterns-operator__secret-form',
  secretConfigAlert: '.pf-v6-c-alert:has-text("Secret Configuration")',

  // Tier filter options
  tierOption: (tier: string) => `.pf-v6-c-menu__list-item:has-text("${tier}")`,
};
