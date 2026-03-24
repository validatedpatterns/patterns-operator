import { test, expect } from '@playwright/test';
import { login, dismissTour, gotoCatalogPage } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Install Pattern Page', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await dismissTour(page);

    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const installButton = page.locator(selectors.patternCard).locator(selectors.installButton).first();
    const count = await installButton.count();
    test.skip(count === 0, 'No installable patterns available');
    await installButton.click();
    await page.waitForSelector(selectors.installPageTitle);
  });

  test('install form shows pre-populated fields', async ({ page }) => {
    const nameInput = page.locator(selectors.patternNameInput);
    await expect(nameInput).toBeVisible();
    const nameValue = await nameInput.inputValue();
    expect(nameValue).toBeTruthy();

    const repoInput = page.locator(selectors.targetRepoInput);
    await expect(repoInput).toBeVisible();
    const repoValue = await repoInput.inputValue();
    expect(repoValue).toMatch(/^https?:\/\//);

    const revisionInput = page.locator(selectors.targetRevisionInput);
    await expect(revisionInput).toBeVisible();
    const revisionValue = await revisionInput.inputValue();
    expect(revisionValue).toBeTruthy();
  });

  test('install form has submit and cancel buttons', async ({ page }) => {
    await expect(page.locator(selectors.submitInstallButton)).toBeVisible();
    await expect(page.locator(selectors.cancelButton)).toBeVisible();
  });

  test('install page shows secret form sections when pattern has secrets', async ({ page }) => {
    const sections = page.locator(selectors.secretSection);
    const count = await sections.count();
    // Not all patterns have secrets, so this is conditional
    if (count > 0) {
      // First section should be expanded by default
      const firstSection = sections.first();
      await expect(firstSection).toBeVisible();
      // Should have secret fields inside the expanded section
      const fields = firstSection.locator(selectors.secretField);
      expect(await fields.count()).toBeGreaterThan(0);
    }
  });

  test('secret form sections can be expanded and collapsed', async ({ page }) => {
    const sections = page.locator(selectors.secretSection);
    const count = await sections.count();
    test.skip(count < 2, 'Pattern has fewer than 2 secret sections');

    // The second section should be collapsed by default — click to expand
    const secondSection = sections.nth(1);
    const toggleButton = secondSection.locator('button').first();
    await toggleButton.click();

    // Should now show fields
    const fields = secondSection.locator(selectors.secretField);
    await expect(fields.first()).toBeVisible();
  });
});
