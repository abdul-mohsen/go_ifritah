import { test, expect } from '@playwright/test';

test.describe('Purchase Bill CSV Import', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/dashboard/purchase-bills/create');
  });

  test('CSV import buttons visible', async ({ page }) => {
    const downloadBtn = page.locator('button[onclick="downloadCSVTemplate()"]');
    const uploadInput = page.locator('input[type="file"][accept=".csv"]');
    
    await expect(downloadBtn).toBeVisible();
    await expect(uploadInput).toBeVisible();
  });

  test('download CSV template', async ({ page, context }) => {
    // Wait for page to fully load
    await page.waitForLoadState('networkidle');
    
    // Click download button
    const downloadBtn = page.locator('button[onclick="downloadCSVTemplate()"]');
    await downloadBtn.click();
    
    // Wait for download to initiate
    await page.waitForTimeout(500);
  });

  test('upload and populate CSV items', async ({ page }) => {
    await page.waitForLoadState('networkidle');
    
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
    await page.waitForLoadState('networkidle');
    
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
