'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const test = require('node:test');
const path = require('node:path');
const validation = require('../static/js/purchase-import-validation.js');

function validBill() {
  return {
    payload: {
      store_id: 1,
      supplier_id: 2,
      supplier_sequence_number: 3,
      payment_method: 10,
      effective_date: '2026-11-05T00:00:00+03:00',
      manual_products: [
        {
          name: 'Brake pad',
          quantity: '2',
          price: '10.00',
          cost_price: '8.00',
        },
      ],
    },
  };
}

test('valid purchase imports produce no validation warning', () => {
  assert.equal(validation.validatePurchaseImportBills([validBill()]), '');
});

test('business-invalid purchase rows return a warning category without mutation', () => {
  const bill = validBill();
  bill.payload.manual_products[0].cost_price = 'not-a-number';
  const before = JSON.stringify(bill);

  assert.equal(validation.validatePurchaseImportBills([bill]), 'product_fields');
  assert.equal(JSON.stringify(bill), before);
});

test('empty product rows are distinct from incomplete product rows', () => {
  assert.equal(validation.hasProductInput(['', '', '', '']), false);
  assert.equal(validation.hasProductInput(['', '2', '', '']), true);
});

test('purchase preview reports validation as a warning without returning early', () => {
  const template = fs.readFileSync(path.join(__dirname, '..', 'templates', 'import-purchase-bills.html'), 'utf8');
  const start = template.indexOf('var validationError =');
  const end = template.indexOf('parsedBillsForImport = bills;', start);
  assert.ok(start >= 0 && end > start, 'preview validation block should exist');

  const validationBlock = template.slice(start, end);
  assert.match(template, /id="preview-validation-warning"/);
  assert.doesNotMatch(validationBlock, /alert\(validationError\)/);
  assert.doesNotMatch(validationBlock, /\breturn\s*;/);
});
