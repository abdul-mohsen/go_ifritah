// Each sale form history entry has its own bill_request_id. Reloads and back/forward
// navigation restore that entry's ID; a newly opened form gets a new ID.
(function () {
  var POS_KEY = 'afrita_pos_id';
  var ATTEMPT_STATE_KEY = '__afrita_sale_identity';
  var posMemory = null;
  var memoryAttempts = {};

  function newUUID() {
    var b = new Uint8Array(16);
    crypto.getRandomValues(b);
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    var h = '';
    for (var value of b) {
      h += ('0' + value.toString(16)).slice(-2);
    }
    return h.slice(0, 8) + '-' + h.slice(8, 12) + '-' + h.slice(12, 16) + '-' + h.slice(16, 20) + '-' + h.slice(20);
  }

  function readStored(name, key) {
    try {
      return window[name].getItem(key);
    } catch (e) {
      return null;
    }
  }

  function writeStored(name, key, value) {
    try {
      window[name].setItem(key, value);
    } catch (e) {}
  }

  function isValidPosId(value) {
    return typeof value === 'string' && value.length > 0 && value.length <= 64 && value === value.trim();
  }

  function isValidAttemptId(value) {
    return typeof value === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);
  }

  function posId() {
    if (!posMemory) {
      var stored = readStored('localStorage', POS_KEY);
      posMemory = isValidPosId(stored) ? stored : newUUID();
      writeStored('localStorage', POS_KEY, posMemory);
    }
    return posMemory;
  }

  function attemptId(scope) {
    var state = null;
    try {
      state = window.history.state;
    } catch (e) {}

    var nextState = state && typeof state === 'object' && !Array.isArray(state) ? Object.assign({}, state) : {};
    var attempts = nextState[ATTEMPT_STATE_KEY];
    attempts = attempts && typeof attempts === 'object' && !Array.isArray(attempts) ? Object.assign({}, attempts) : {};
    if (isValidAttemptId(attempts[scope])) {
      return attempts[scope];
    }

    if (isValidAttemptId(memoryAttempts[scope])) return memoryAttempts[scope];

    var value = newUUID();
    attempts[scope] = value;
    nextState[ATTEMPT_STATE_KEY] = attempts;
    try {
      window.history.replaceState(nextState, '');
    } catch (e) {
      memoryAttempts[scope] = value;
    }
    return value;
  }

  function scopeOf(evt) {
    return evt.detail.elt?.dataset?.saleScope;
  }

  function initializeForms() {
    for (var form of document.querySelectorAll('[data-sale-scope]')) {
      attemptId(form.dataset.saleScope);
    }
  }

  document.addEventListener('htmx:configRequest', function (evt) {
    var scope = scopeOf(evt);
    if (!scope) return;
    evt.detail.parameters.pos_id = posId();
    evt.detail.parameters.bill_request_id = attemptId(scope);
  });

  document.addEventListener('htmx:afterSwap', initializeForms);
  document.addEventListener('htmx:pushedIntoHistory', function () {
    memoryAttempts = {};
    initializeForms();
  });

  window.addEventListener('popstate', function () {
    memoryAttempts = {};
    initializeForms();
  });

  window.addEventListener('pageshow', initializeForms);
  initializeForms();
})();
