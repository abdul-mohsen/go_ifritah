'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createPage, createStorage, loadStatic } = require('./support.js');

function openSalePage({ localStorage, sessionStorage, navigationType } = {}) {
  const form = { dataset: { saleScope: 'sale' } };
  const page = createPage({ elements: [form], localStorage, sessionStorage, navigationType });
  loadStatic('js/sale-identity.js', page);
  return { page, form };
}

function submit({ page, form }) {
  const evt = { detail: { elt: form, parameters: {} } };
  page.document.dispatch('htmx:configRequest', evt);
  return evt.detail.parameters;
}

function answer({ page, form }, status) {
  page.document.dispatch('htmx:afterRequest', { detail: { elt: form, xhr: { status } } });
}

// Guards the B6 fix: a normal navigation keeps the stored attempt, so only a reload may start a new one.
test('normal navigation keeps the pos_id and the sale attempt', () => {
  const localStorage = createStorage();
  const sessionStorage = createStorage();
  const before = submit(openSalePage({ localStorage, sessionStorage }));
  const after = submit(openSalePage({ localStorage, sessionStorage }));

  assert.ok(before.pos_id);
  assert.ok(before.bill_request_id);
  assert.equal(after.pos_id, before.pos_id);
  assert.equal(after.bill_request_id, before.bill_request_id);
});

test('a reload starts a new sale attempt instead of reusing the stored one', () => {
  const localStorage = createStorage();
  const sessionStorage = createStorage();
  const before = submit(openSalePage({ localStorage, sessionStorage }));
  const after = submit(openSalePage({ localStorage, sessionStorage, navigationType: 'reload' }));

  assert.ok(after.bill_request_id);
  assert.notEqual(after.bill_request_id, before.bill_request_id);
  assert.notEqual(sessionStorage.getItem('afrita_sale_attempt:sale'), before.bill_request_id);
});

test('two submits before any answer share one bill_request_id', () => {
  const sale = openSalePage();
  const first = submit(sale);
  const second = submit(sale);

  assert.ok(first.bill_request_id);
  assert.equal(second.bill_request_id, first.bill_request_id);
});

test('a failed attempt keeps its bill_request_id on retry', () => {
  const sale = openSalePage();
  const first = submit(sale);
  answer(sale, 500);
  const retry = submit(sale);

  assert.equal(retry.bill_request_id, first.bill_request_id);
});
