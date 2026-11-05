'use strict';

const { webcrypto } = require('node:crypto');

const installers = new Map([
  ['js/sale-identity.js', require('../static/js/sale-identity.js')],
  ['js/sale-discount.js', require('../static/js/sale-discount.js')],
]);
const NAVIGATION_TYPES = { navigate: 0, reload: 1, back_forward: 2 };
const SELECTOR = /^\[data-([a-z0-9-]+)(?:="([^"]*)")?\]$/;

function createStorage() {
  const items = new Map();
  return {
    getItem(key) {
      return items.has(key) ? items.get(key) : null;
    },
    setItem(key, value) {
      items.set(key, String(value));
    },
    removeItem(key) {
      items.delete(key);
    },
    clear() {
      items.clear();
    },
    key(index) {
      return index < items.size ? [...items.keys()][index] : null;
    },
    get length() {
      return items.size;
    },
  };
}

function createEventTarget() {
  const listeners = new Map();
  return {
    addEventListener(type, listener) {
      listeners.set(type, [...(listeners.get(type) || []), listener]);
    },
    dispatch(type, evt) {
      for (const listener of listeners.get(type) || []) {
        listener(evt);
      }
    },
  };
}

function matcherFor(selector) {
  const match = SELECTOR.exec(selector);
  if (!match) {
    throw new Error(`js-tests harness supports only [data-x] and [data-x="v"], got: ${selector}`);
  }
  const key = match[1].replace(/-([a-z0-9])/g, (_, c) => c.toUpperCase());
  const expected = match[2];
  return (element) => {
    const value = element.dataset?.[key];
    return value !== undefined && (expected === undefined || value === expected);
  };
}

function createDocument(elements) {
  return Object.assign(createEventTarget(), {
    querySelectorAll(selector) {
      return elements.filter(matcherFor(selector));
    },
  });
}

function createPage({
  navigationType = 'navigate',
  elements = [],
  localStorage = createStorage(),
  sessionStorage = createStorage(),
  historyState = null,
} = {}) {
  if (!(navigationType in NAVIGATION_TYPES)) {
    throw new Error(`unknown navigationType: ${navigationType}`);
  }
  const window = Object.assign(createEventTarget(), {
    document: createDocument(elements),
    localStorage,
    sessionStorage,
    history: {
      state: historyState,
      replaceState(state) {
        this.state = state;
      },
    },
    performance: {
      getEntriesByType: (type) => (type === 'navigation' ? [{ type: navigationType }] : []),
      navigation: { type: NAVIGATION_TYPES[navigationType] },
    },
    crypto: webcrypto,
  });
  window.window = window;
  return { window, document: window.document, localStorage, sessionStorage, performance: window.performance };
}

function loadStatic(script, page) {
  const installer = installers.get(script);
  if (!installer) {
    throw new Error(`unsupported static script: ${script}`);
  }
  installer(page.window);
}

module.exports = { createPage, createStorage, loadStatic };
