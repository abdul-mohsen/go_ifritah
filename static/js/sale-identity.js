// Identity of a final sale. Every submit of a sale form sends two values, and the backend needs both on a final sale:
//   pos_id          names this browser's point of sale. It is created on first use and kept in localStorage.
//   bill_request_id names one sale attempt. A reload before the answer, a retry, or a double click sends the same value.
//                   It is kept in sessionStorage until the sale is saved (2xx), and it is dropped on a 409. A page keeps
//                   the value it used, so a second submit from the same page is answered with the original sale. The next
//                   page starts a new sale with a new value.
// If storage is blocked, the values are kept in memory for the current page only.
(function () {
  var POS_KEY = 'afrita_pos_id';
  var ATTEMPT_PREFIX = 'afrita_sale_attempt:';
  var posMemory = null;
  var current = {};

  function newUUID() {
    var b = new Uint8Array(16);
    crypto.getRandomValues(b);
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    var h = '';
    for (var i = 0; i < b.length; i++) {
      h += ('0' + b[i].toString(16)).slice(-2);
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

  function removeStored(name, key) {
    try {
      window[name].removeItem(key);
    } catch (e) {}
  }

  function isValidPosId(value) {
    return typeof value === 'string' && value.length > 0 && value.length <= 64 && value === value.trim();
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
    var key = ATTEMPT_PREFIX + scope;
    if (!current[scope]) {
      current[scope] = readStored('sessionStorage', key) || newUUID();
      writeStored('sessionStorage', key, current[scope]);
    }
    return current[scope];
  }

  function scopeOf(evt) {
    var elt = evt.detail.elt;
    return elt && elt.getAttribute ? elt.getAttribute('data-sale-scope') : null;
  }

  document.addEventListener('htmx:configRequest', function (evt) {
    var scope = scopeOf(evt);
    if (!scope) return;
    evt.detail.parameters.pos_id = posId();
    evt.detail.parameters.bill_request_id = attemptId(scope);
  });

  document.addEventListener('htmx:afterRequest', function (evt) {
    var scope = scopeOf(evt);
    if (!scope) return;
    // htmx leaves detail.successful unset for an HX-Redirect answer, which is how a saved sale is answered, so the
    // status decides.
    var status = evt.detail.xhr.status;
    if (status >= 200 && status < 300) {
      removeStored('sessionStorage', ATTEMPT_PREFIX + scope);
    } else if (status === 409) {
      delete current[scope];
      removeStored('sessionStorage', ATTEMPT_PREFIX + scope);
    }
  });

  // A page restored from the back-forward cache may still hold a value from a sale that was saved after it was shown.
  window.addEventListener('pageshow', function (evt) {
    if (evt.persisted) current = {};
  });

  // A sale form gets its value when it opens, so the value exists before the first submit.
  var forms = document.querySelectorAll('[data-sale-scope]');
  for (var i = 0; i < forms.length; i++) {
    attemptId(forms[i].getAttribute('data-sale-scope'));
  }
})();
