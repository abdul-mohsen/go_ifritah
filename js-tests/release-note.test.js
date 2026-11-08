'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createStorage } = require('./support.js');
const installReleaseNote = require('../static/js/release-note.js');

class FakeClassList {
  constructor(...initial) {
    this.values = new Set(initial);
  }

  add(...names) {
    names.forEach((name) => this.values.add(name));
  }

  remove(...names) {
    names.forEach((name) => this.values.delete(name));
  }

  contains(name) {
    return this.values.has(name);
  }
}

class FakeElement {
  constructor(document, { id, classes = [], attributes = {}, tagName = 'DIV' } = {}) {
    this.ownerDocument = document;
    this.id = id || '';
    this.tagName = tagName;
    this.classList = new FakeClassList(...classes);
    this.attributes = { ...attributes };
    this.dataset = {};
    this.listeners = new Map();
    Object.entries(attributes).forEach(([name, value]) => {
      if (name.startsWith('data-')) {
        const key = name.slice(5).replace(/-([a-z])/g, (_, character) => character.toUpperCase());
        this.dataset[key] = value;
      }
    });
  }

  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }

  getAttribute(name) {
    return this.attributes[name] ?? null;
  }

  addEventListener(type, listener) {
    this.listeners.set(type, [...(this.listeners.get(type) || []), listener]);
  }

  listenerCount(type) {
    return (this.listeners.get(type) || []).length;
  }

  dispatch(type, event = {}) {
    const dispatched = {
      target: this,
      preventDefault() {
        this.defaultPrevented = true;
      },
      ...event,
    };
    for (const listener of this.listeners.get(type) || []) {
      listener(dispatched);
    }
    return dispatched;
  }

  focus() {
    this.ownerDocument.activeElement = this;
  }

  querySelectorAll(selector) {
    if (selector === '[data-release-note-dismiss]' || selector.includes('button')) {
      return this.ownerDocument.dismissButtons;
    }
    return [];
  }
}

class FakeDocument {
  constructor({ version, storage }) {
    this.readyState = 'loading';
    this.activeElement = null;
    this.listeners = new Map();
    this.storage = storage;
    this._cookie = '';
    this.body = new FakeElement(this, { id: 'body' });
    this.releaseNote = new FakeElement(this, {
      id: 'afrita-release-note',
      classes: ['modal-overlay', 'hidden'],
      attributes: {
        'data-release-note-version': version,
        'aria-hidden': 'true',
      },
    });
    this.closeButton = new FakeElement(this, {
      tagName: 'BUTTON',
      attributes: { 'data-release-note-dismiss': '' },
    });
    this.continueButton = new FakeElement(this, {
      tagName: 'BUTTON',
      attributes: { 'data-release-note-dismiss': '' },
    });
    this.dismissButtons = [this.closeButton, this.continueButton];
    this.activeElement = this.body;
  }

  get cookie() {
    return this._cookie;
  }

  set cookie(value) {
    this._cookie = this._cookie ? `${this._cookie}; ${value.split(';')[0]}` : value;
  }

  getElementById(id) {
    return id === 'afrita-release-note' ? this.releaseNote : null;
  }

  addEventListener(type, listener) {
    this.listeners.set(type, [...(this.listeners.get(type) || []), listener]);
  }

  dispatch(type, event = {}) {
    const dispatched = {
      type,
      target: this,
      preventDefault() {
        this.defaultPrevented = true;
      },
      ...event,
    };
    for (const listener of this.listeners.get(type) || []) {
      listener(dispatched);
    }
    return dispatched;
  }
}

function createReleaseNotePage({ storage = createStorage(), version = 'v0.1.0' } = {}) {
  const document = new FakeDocument({ storage, version });
  const window = {
    document,
    localStorage: storage,
  };
  const controller = installReleaseNote(window);
  document.dispatch('DOMContentLoaded');
  return { controller, document, storage, window };
}

test('first app open shows the localized release note and moves focus into the dialog', () => {
  const page = createReleaseNotePage();

  assert.equal(page.controller.isOpen(), true);
  assert.equal(page.document.releaseNote.getAttribute('aria-hidden'), 'false');
  assert.equal(page.document.releaseNote.classList.contains('hidden'), false);
  assert.equal(page.document.activeElement, page.document.closeButton);
  assert.equal(page.storage.getItem(page.controller.storageKey()), null);
});

test('dismissal is persisted and prevents the release note on later navigation', () => {
  const storage = createStorage();
  const first = createReleaseNotePage({ storage });

  first.document.closeButton.dispatch('click');
  assert.equal(first.controller.isOpen(), false);
  assert.equal(storage.getItem('afrita.release-note.dismissed.v0.1.0'), '1');
  assert.equal(first.document.body.classList.contains('release-note-open'), false);

  const later = createReleaseNotePage({ storage });
  assert.equal(later.controller.isOpen(), false);
});

test('Escape dismisses once and repeated HTMX swaps do not duplicate handlers', () => {
  const page = createReleaseNotePage();

  page.document.activeElement = page.document.continueButton;
  page.document.dispatch('htmx:afterSwap');
  page.document.dispatch('htmx:afterSwap');
  assert.equal(page.document.closeButton.listenerCount('click'), 1);
  assert.equal(page.document.activeElement, page.document.continueButton);

  const event = page.document.dispatch('keydown', { key: 'Escape' });
  assert.equal(event.defaultPrevented, true);
  assert.equal(page.controller.isOpen(), false);
  assert.equal(page.storage.getItem(page.controller.storageKey()), '1');
  assert.equal(page.document.activeElement, page.document.body);

  page.document.dispatch('htmx:afterSwap');
  assert.equal(page.controller.isOpen(), false);
});

test('focus stays inside the release note while it is open', () => {
  const page = createReleaseNotePage();

  page.document.activeElement = page.document.continueButton;
  const forward = page.document.dispatch('keydown', { key: 'Tab' });
  assert.equal(forward.defaultPrevented, true);
  assert.equal(page.document.activeElement, page.document.closeButton);

  page.document.activeElement = page.document.closeButton;
  const backward = page.document.dispatch('keydown', { key: 'Tab', shiftKey: true });
  assert.equal(backward.defaultPrevented, true);
  assert.equal(page.document.activeElement, page.document.continueButton);
});

test('release-note version namespaces dismissal so the public MVP release can be announced', () => {
  const storage = createStorage();
  storage.setItem('afrita.release-note.dismissed.v0.0.2', '1');

  const page = createReleaseNotePage({ storage, version: 'v0.1.0' });

  assert.equal(page.controller.storageKey(), 'afrita.release-note.dismissed.v0.1.0');
  assert.equal(page.controller.isOpen(), true);
});
