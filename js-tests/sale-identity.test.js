'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createPage, createStorage, loadStatic } = require('./support.js');

function openSalePage({ localStorage, sessionStorage, navigationType, historyState } = {}) {
  const form = { dataset: { saleScope: 'sale' } };
  const page = createPage({ elements: [form], localStorage, sessionStorage, navigationType, historyState });
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

test('reload keeps the same sale attempt for the same history entry', () => {
  const localStorage = createStorage();
  const sessionStorage = createStorage();
  const sale = openSalePage({ localStorage, sessionStorage });
  const before = submit(sale);
  const reload = openSalePage({
    localStorage,
    sessionStorage,
    navigationType: 'reload',
    historyState: sale.page.window.history.state,
  });
  const after = submit(reload);

  assert.ok(before.pos_id);
  assert.ok(before.bill_request_id);
  assert.equal(after.pos_id, before.pos_id);
  assert.equal(after.bill_request_id, before.bill_request_id);
  assert.equal(sessionStorage.length, 0);
});

test('a new sale navigation entry gets a fresh attempt after abandoning the prior form', () => {
  const localStorage = createStorage();
  const before = submit(openSalePage({ localStorage }));
  const after = submit(openSalePage({ localStorage }));

  assert.equal(after.pos_id, before.pos_id);
  assert.notEqual(after.bill_request_id, before.bill_request_id);
});

test('back navigation restores the attempt belonging to the prior entry', () => {
  const localStorage = createStorage();
  const firstEntry = openSalePage({ localStorage });
  const first = submit(firstEntry);
  const second = submit(openSalePage({ localStorage }));

  const restored = openSalePage({
    localStorage,
    navigationType: 'back_forward',
    historyState: firstEntry.page.window.history.state,
  });

  assert.notEqual(second.bill_request_id, first.bill_request_id);
  assert.equal(submit(restored).bill_request_id, first.bill_request_id);
});

test('two submits before any answer share one bill_request_id', () => {
  const sale = openSalePage();
  const first = submit(sale);
  const second = submit(sale);

  assert.ok(first.bill_request_id);
  assert.equal(second.bill_request_id, first.bill_request_id);
});

test('failed, successful, and conflict responses retain the same attempt for retry', () => {
  const sale = openSalePage();
  const first = submit(sale);

  answer(sale, 500);
  assert.equal(submit(sale).bill_request_id, first.bill_request_id);
  answer(sale, 201);
  assert.equal(submit(sale).bill_request_id, first.bill_request_id);
  answer(sale, 409);
  assert.equal(submit(sale).bill_request_id, first.bill_request_id);
});

test('separate tabs get separate attempts while sharing the browser POS id', () => {
  const localStorage = createStorage();
  const tabA = submit(openSalePage({ localStorage }));
  const tabB = submit(openSalePage({ localStorage }));

  assert.equal(tabB.pos_id, tabA.pos_id);
  assert.notEqual(tabB.bill_request_id, tabA.bill_request_id);
});

test('sale identity is namespaced without discarding unrelated history state', () => {
  const originalState = { unrelated: 'preserve-me' };
  const sale = openSalePage({ historyState: originalState });
  const identity = submit(sale).bill_request_id;
  const state = sale.page.window.history.state;

  assert.equal(state.unrelated, 'preserve-me');
  assert.equal(state.__afrita_sale_identity.sale, identity);
});

test('static script loader rejects installers outside its allowlist', () => {
  const page = createPage();

  assert.throws(() => loadStatic('js/unknown.js', page), /unsupported static script/);
});
