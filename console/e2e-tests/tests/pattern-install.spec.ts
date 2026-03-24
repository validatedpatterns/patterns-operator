import { test, expect } from '@playwright/test';
import { login, dismissTour } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Install Pattern Page', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await dismissTour(page);
    await page.goto('/patterns');
    await page.waitForSelector(selectors.spinner, { state: 'detached', timeout: 30_000 });

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
});
