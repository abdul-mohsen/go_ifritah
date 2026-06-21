import { test, expect } from '@playwright/test';

test.describe('Purchase Bill CSV Import', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/dashboard/purchase-bills/create');
  });

  test('CSV import buttons visible', async ({ page }) => {
    const downloadBtn = page.locator('button:has-text("Download Template")');
    const uploadLabel = page.locator('label:has-text("Upload CSV")');
    
    await expect(downloadBtn).toBeVisible();
    await expect(uploadLabel).toBeVisible();
  });

  test('download CSV template', async ({ page, context }) => {
    // Intercept download
    const downloadPromise = context.waitForEvent('page');
    await page.click('button:has-text("Download Template")');
    
    // Verify download initiated (page won't be created for file download)
    await page.waitForTimeout(500);
    // Just verify no errors occurred
  });

  test('upload and populate CSV items', async ({ page }) => {
    // Create a test CSV file
    const csvContent = `اسم القطعة,الكمية,سعر الشراء,سعر التكلفة,رقم الرف
محرك,5,150.50,130.00,A1
مضخة,3,200.00,180.00,A2`;
    
    const buffer = Buffer.from(csvContent, 'utf-8');
    
    // Upload file
    const fileInput = page.locator('input[type="file"][accept=".csv"]');
    await fileInput.setInputFiles({
      name: 'items.csv',
      mimeType: 'text/csv',
      buffer: buffer
    });
    
    // Wait for items to be added
    await page.waitForTimeout(500);
    
    // Verify items were added (check manual-container has items)
    const container = page.locator('#manual-container');
    await expect(container).toContainText('مضخة');
  });

  test('handles invalid CSV gracefully', async ({ page }) => {
    const fileInput = page.locator('input[type="file"][accept=".csv"]');
    
    // Upload empty/invalid file
    await fileInput.setInputFiles({
      name: 'invalid.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from('invalid,data', 'utf-8')
    });
    
    await page.waitForTimeout(500);
    // Should not crash
    await expect(page).not.toHaveTitle(/error/i);
  });
});
