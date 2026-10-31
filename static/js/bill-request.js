(function (root, factory) {
  if (typeof module === 'object' && module.exports) {
    module.exports = factory();
    return;
  }

  root.BillRequest = factory(root);
  root.BillRequest.install({ window: root, document: root.document });
})(typeof globalThis !== 'undefined' ? globalThis : this, function (browserRoot) {
  var STORAGE_PREFIX = 'afrita:bill-request:v1:';

  function defaultGenerateId() {
    if (browserRoot && browserRoot.crypto) {
      if (typeof browserRoot.crypto.randomUUID === 'function') {
        return browserRoot.crypto.randomUUID();
      }
      if (typeof browserRoot.crypto.getRandomValues === 'function') {
        var bytes = new Uint8Array(16);
        browserRoot.crypto.getRandomValues(bytes);
        bytes[6] = (bytes[6] & 0x0f) | 0x40;
        bytes[8] = (bytes[8] & 0x3f) | 0x80;
        var hex = Array.from(bytes, function (byte) {
          return byte.toString(16).padStart(2, '0');
        }).join('');
        return [hex.slice(0, 8), hex.slice(8, 12), hex.slice(12, 16), hex.slice(16, 20), hex.slice(20)].join('-');
      }
    }

    return 'bill-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2);
  }

  function createController(options) {
    options = options || {};
    var storage = options.storage || null;
    var generateId = options.generateId || defaultGenerateId;
    var now = options.now || Date.now;
    var inFlight = Object.create(null);
    var memoryStorage = Object.create(null);

    function keyForScope(scope) {
      return STORAGE_PREFIX + encodeURIComponent(scope || 'default');
    }

    function read(scope) {
      var key = keyForScope(scope);
      var raw = null;
      try {
        raw = storage ? storage.getItem(key) : memoryStorage[key];
      } catch (e) {
        raw = memoryStorage[key] || null;
      }
      if (!raw) return null;

      try {
        var value = typeof raw === 'string' ? JSON.parse(raw) : raw;
        if (!value || typeof value.id !== 'string' || value.id === '') return null;
        return {
          id: value.id,
          status: value.status || 'ready',
          updatedAt: value.updatedAt || 0,
        };
      } catch (e) {
        return null;
      }
    }

    function write(scope, value) {
      var key = keyForScope(scope);
      var raw = JSON.stringify(value);
      memoryStorage[key] = raw;
      try {
        if (storage) storage.setItem(key, raw);
      } catch (e) {}
    }

    function remove(scope) {
      var key = keyForScope(scope);
      delete memoryStorage[key];
      try {
        if (storage) storage.removeItem(key);
      } catch (e) {}
    }

    function getOrCreate(scope) {
      var value = read(scope);
      if (value) return value;

      value = {
        id: generateId(),
        status: 'ready',
        updatedAt: now(),
      };
      write(scope, value);
      return value;
    }

    function prepare(scope) {
      var value = getOrCreate(scope);
      if (value.status === 'in_flight' && !inFlight[scope]) {
        value.status = 'unknown';
        value.updatedAt = now();
        write(scope, value);
      }
      return value;
    }

    function begin(scope) {
      var value = getOrCreate(scope);
      if (inFlight[scope] || value.status === 'in_flight') {
        return {
          accepted: false,
          reason: 'in-flight',
          id: value.id,
        };
      }

      inFlight[scope] = true;
      value.status = 'in_flight';
      value.updatedAt = now();
      write(scope, value);
      return {
        accepted: true,
        id: value.id,
      };
    }

    function finish(scope, outcome) {
      delete inFlight[scope];
      var value = read(scope);
      if (!value) return;

      if (outcome === 'success') {
        remove(scope);
        return;
      }

      value.status = outcome === 'unknown' ? 'unknown' : 'ready';
      value.updatedAt = now();
      write(scope, value);
    }

    return {
      begin: begin,
      finish: finish,
      getOrCreate: getOrCreate,
      isInFlight: function (scope) {
        var value = read(scope);
        return !!inFlight[scope] || (value && value.status === 'in_flight');
      },
      keyForScope: keyForScope,
      peek: function (scope) {
        return read(scope);
      },
      prepare: prepare,
      clear: function (scope) {
        delete inFlight[scope];
        remove(scope);
      },
    };
  }

  function install(options) {
    options = options || {};
    var win = options.window || browserRoot;
    var doc = options.document || (win && win.document);
    if (!doc) return null;
    if (doc.__afritaBillRequestController) return doc.__afritaBillRequestController;

    var storage = options.storage;
    if (!storage && win) {
      try {
        storage = win.localStorage;
      } catch (e) {}
    }
    var controller =
      options.controller ||
      createController({
        storage: storage,
        generateId: options.generateId,
        now: options.now,
      });

    function closest(element, selector) {
      if (!element) return null;
      if (typeof element.closest === 'function') return element.closest(selector);
      return null;
    }

    function contextFor(element) {
      if (!element) return null;
      var form = element.tagName === 'FORM' ? element : element.form;
      if (!form) form = closest(element, 'form');
      var root = closest(element, '[data-bill-submit-form], [data-bill-submit-button]');
      var state = closest(element, '[data-bill-submit-state]');
      if (!root && state) {
        root = state.parentElement && state.parentElement.querySelector('[data-bill-submit-button]');
      }
      if (!root && form && form.hasAttribute && form.hasAttribute('data-bill-submit-form')) {
        root = form;
      }
      if (!root) return null;

      var scope =
        (state && state.getAttribute('data-bill-submit-scope')) ||
        root.getAttribute('data-bill-request-scope') ||
        (form && form.getAttribute('data-bill-request-scope')) ||
        (win && win.location && win.location.pathname) ||
        'pos';
      return {
        form: form,
        root: root,
        scope: scope,
      };
    }

    function stateElement() {
      return doc.querySelector('[data-bill-submit-state]');
    }

    function syncHiddenField(context, id) {
      if (!context.form) return;
      var field = context.form.querySelector('input[name="bill_request_id"]');
      if (field) field.value = id;
    }

    function controlsFor(context) {
      if (context.form) {
        return Array.from(context.form.querySelectorAll('[data-bill-submit-button]'));
      }
      return [context.root];
    }

    function setState(context, state) {
      var stateEl = stateElement();
      if (!stateEl) return;

      stateEl.setAttribute('data-bill-submit-state', state);
      stateEl.setAttribute(
        'data-bill-submit-result',
        state === 'unknown' ? 'result-unknown' : state === 'in_flight' ? 'waiting' : 'idle'
      );
      stateEl.hidden = state === 'idle';

      var message = stateEl.querySelector('[data-bill-submit-message]');
      var retry = stateEl.querySelector('[data-bill-submit-retry]');
      var unknownMessage =
        stateEl.getAttribute('data-unknown-message') ||
        'نتيجة إرسال الفاتورة غير معروفة. لا تنشئ فاتورة بديلة؛ أعد المحاولة بنفس الفاتورة.';
      var waitingMessage = stateEl.getAttribute('data-waiting-message') || 'جارٍ إرسال الفاتورة، يرجى الانتظار.';

      if (message) {
        message.textContent = state === 'unknown' ? unknownMessage : waitingMessage;
      }
      if (retry) {
        retry.hidden = state !== 'unknown';
      }

      controlsFor(context).forEach(function (button) {
        if (button === retry) return;
        button.hidden = state === 'unknown';
        button.disabled = state === 'in_flight';
      });
    }

    function recoverOnLoad() {
      var elements = doc.querySelectorAll('[data-bill-submit-form], [data-bill-submit-button]');
      elements.forEach(function (element) {
        var context = contextFor(element);
        if (!context) return;
        var value = controller.peek(context.scope);
        if (!value) return;
        if (value.status === 'in_flight') value = controller.prepare(context.scope);
        syncHiddenField(context, value.id);
        if (value.status === 'unknown') setState(context, 'unknown');
      });
    }

    function markUnknown(context) {
      if (!context) return;
      controller.getOrCreate(context.scope);
      controller.finish(context.scope, 'unknown');
      var value = controller.getOrCreate(context.scope);
      syncHiddenField(context, value.id);
      setState(context, 'unknown');
    }

    function requestContext(evt) {
      var detail = evt && evt.detail;
      return contextFor((detail && (detail.elt || detail.target)) || (evt && evt.target));
    }

    doc.addEventListener('htmx:configRequest', function (evt) {
      var context = requestContext(evt);
      if (!context || !evt.detail) return;

      var value = controller.getOrCreate(context.scope);
      syncHiddenField(context, value.id);
      var parameters = evt.detail.parameters;
      if (parameters && typeof parameters === 'object') {
        if (typeof parameters.set === 'function') {
          parameters.set('bill_request_id', value.id);
        } else {
          parameters.bill_request_id = value.id;
        }
      }
    });

    doc.addEventListener('htmx:beforeRequest', function (evt) {
      var context = requestContext(evt);
      if (!context) return;

      var result = controller.begin(context.scope);
      if (!result.accepted) {
        evt.preventDefault();
        setState(context, 'unknown');
        return;
      }

      syncHiddenField(context, result.id);
      setState(context, 'in_flight');
    });

    doc.addEventListener('htmx:afterRequest', function (evt) {
      var context = requestContext(evt);
      if (!context) return;

      var xhr = evt.detail && evt.detail.xhr;
      var status = xhr ? xhr.status : 0;
      if (status >= 200 && status < 400) {
        controller.finish(context.scope, 'success');
        setState(context, 'idle');
      } else if (status === 0 || status >= 500) {
        markUnknown(context);
      } else {
        controller.finish(context.scope, 'retryable');
        setState(context, 'idle');
      }
    });

    doc.addEventListener('htmx:sendError', function (evt) {
      markUnknown(requestContext(evt));
    });

    doc.addEventListener('htmx:timeout', function (evt) {
      markUnknown(requestContext(evt));
    });

    doc.addEventListener('htmx:responseError', function (evt) {
      var context = requestContext(evt);
      var xhr = evt.detail && evt.detail.xhr;
      if (!context) return;
      if (!xhr || xhr.status === 0 || xhr.status >= 500) {
        markUnknown(context);
      }
    });

    doc.addEventListener('click', function (evt) {
      var retry = closest(evt.target, '[data-bill-submit-retry]');
      if (!retry) return;
      evt.preventDefault();

      var context = contextFor(retry);
      if (!context) return;
      controller.finish(context.scope, 'retryable');
      setState(context, 'idle');

      var submit = context.form ? context.form.querySelector('[data-bill-submit-button][value="1"]') : context.root;
      if (submit && submit !== retry && typeof submit.click === 'function') {
        submit.click();
      }
    });

    doc.__afritaBillRequestController = controller;
    recoverOnLoad();
    return controller;
  }

  return {
    STORAGE_PREFIX: STORAGE_PREFIX,
    createController: createController,
    install: install,
  };
});
