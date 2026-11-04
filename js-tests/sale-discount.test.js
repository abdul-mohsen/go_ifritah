'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createPage, loadStatic } = require('./support.js');

function loadHelperPage() {
  const page = createPage();
  loadStatic('js/sale-discount.js', page);
  return page;
}

function field(name, value, extra = {}) {
  return { name, value, disabled: false, dataset: {}, ...extra };
}

function lineRow(prefix, { price, qty, discount, mode, disabled = false }) {
  return [
    field(`${prefix}_price`, price),
    field(`${prefix}_quantity`, qty),
    field(`${prefix}_discount`, discount, { disabled, dataset: mode ? { mode } : {} }),
  ];
}

function requestParameters(rows) {
  const form = { dataset: { saleDiscounts: '' }, elements: rows.flat() };
  const evt = { detail: { elt: form, parameters: {} } };
  loadHelperPage().document.dispatch('htmx:configRequest', evt);
  return evt.detail.parameters;
}

test('amount converts a value or percent discount into SAR for its line', () => {
  const { amount } = loadHelperPage().window.AfritaSaleDiscount;

  assert.equal(amount('5', 'val', 20), 5);
  assert.equal(amount('25', 'pct', 20), 5);
  assert.equal(amount('33.333', 'pct', 10), 3.33);
});

test('amount caps the discount at the line total and treats bad input as zero', () => {
  const { amount } = loadHelperPage().window.AfritaSaleDiscount;

  assert.equal(amount('50', 'val', 20), 20);
  assert.equal(amount('150', 'pct', 20), 20);
  assert.equal(amount('-5', 'val', 20), 0);
  assert.equal(amount('abc', 'val', 20), 0);
  assert.equal(amount('5', 'val', NaN), 0);
});

test('htmx sends one discount per product and manual row, in row order', () => {
  const params = requestParameters([
    lineRow('products', { price: '20', qty: '1', discount: '5' }),
    lineRow('products', { price: '10', qty: '1', discount: '0' }),
    lineRow('manual', { price: '10', qty: '1', discount: '1.5' }),
  ]);

  assert.deepEqual([...params.products_discount], ['5', '0']);
  assert.deepEqual([...params.manual_discount], ['1.5']);
});

test('a percent discount is sent as the SAR amount of its line', () => {
  const params = requestParameters([lineRow('products', { price: '20', qty: '2', discount: '25', mode: 'pct' })]);

  assert.deepEqual([...params.products_discount], ['10']);
});

// Rows are matched by position, so a disabled discount must keep its slot.
test('a disabled discount keeps its line slot', () => {
  const params = requestParameters([
    lineRow('products', { price: '20', qty: '1', discount: '5', disabled: true }),
    lineRow('products', { price: '10', qty: '1', discount: '2' }),
  ]);

  assert.deepEqual([...params.products_discount], ['5', '2']);
});

test('forms without the sale-discount marker keep their request parameters', () => {
  const page = loadHelperPage();
  const evt = { detail: { elt: { dataset: {}, elements: [field('products_discount', '5')] }, parameters: {} } };
  page.document.dispatch('htmx:configRequest', evt);

  assert.deepEqual(evt.detail.parameters, {});
});
