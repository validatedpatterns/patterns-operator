export const selectors = {
  // Navigation
  navHome: '[data-quickstart-id="qs-nav-home"]',
  navCatalog: '[data-test="nav"] >> text=Catalog',

  // PatternCatalogPage
  catalogPageTitle: 'h1:has-text("Pattern Catalog")',
  tierFilterToggle: '#tier-filter',
  patternCard: '.patterns-operator__card',
  installedLabel: '.patterns-operator__installed-label',
  installButton: 'button:has-text("Install")',
  uninstallButton: 'button:has-text("Uninstall")',
  manageSecretsButton: 'button:has-text("Manage Secrets")',
  docsLink: 'a:has-text("Docs")',
  repoLink: 'a:has-text("Repo")',
  cloudLabel: '.patterns-operator__cloud-labels .pf-v6-c-label',
  spinner: '.pf-v6-c-spinner',

  // InstallPatternPage
  installPageTitle: 'h1:has-text("Install Pattern")',
  patternNameInput: '#pattern-name',
  targetRepoInput: '#pattern-target-repo',
  targetRevisionInput: '#pattern-target-revision',
  submitInstallButton: 'button[type="submit"]:has-text("Install")',
  cancelButton: 'button:has-text("Cancel")',

  // UninstallPatternPage
  uninstallPageTitle: 'h1:has-text("Uninstall Pattern")',
  confirmUninstallButton: 'button:has-text("Confirm Uninstall")',

  // ManageSecretsPage
  manageSecretsPageTitle: 'h1:has-text("Manage Secrets")',
  injectSecretsButton: 'button:has-text("Inject Secrets")',

  // Alerts
  alertDanger: '.pf-v6-c-alert.pf-m-danger',

  // Tier filter options
  tierOption: (tier: string) => `[role="option"]:has-text("${tier}")`,
};
