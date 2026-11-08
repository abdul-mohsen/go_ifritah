// Show the MVP release note once per browser and release-note version.
(function (root, install) {
  if (typeof module === 'object' && module.exports) {
    module.exports = install;
  } else {
    install(root);
  }
})(typeof window !== 'undefined' ? window : null, function installReleaseNote(window) {
  var document = window && window.document;
  var MVP_RELEASE_NOTE_VERSION = 'v0.1.0';
  var STORAGE_PREFIX = 'afrita.release-note.dismissed.';
  var COOKIE_MAX_AGE = 31536000;

  if (!document || window.__afritaReleaseNoteInitialized) {
    return window && window.__afritaReleaseNoteController;
  }

  window.__afritaReleaseNoteInitialized = true;

  var modal = null;
  var previousFocus = null;
  var dismissedInMemory = false;

  function findModal() {
    return document.getElementById('afrita-release-note');
  }

  function releaseNoteVersion() {
    if (!modal) return MVP_RELEASE_NOTE_VERSION;
    return modal.getAttribute('data-release-note-version') || MVP_RELEASE_NOTE_VERSION;
  }

  function storageKey() {
    return STORAGE_PREFIX + releaseNoteVersion();
  }

  function readStorage() {
    try {
      return window.localStorage.getItem(storageKey());
    } catch (e) {
      return null;
    }
  }

  function writeStorage() {
    try {
      window.localStorage.setItem(storageKey(), '1');
    } catch (e) {}
  }

  function readCookie() {
    try {
      var escapedKey = storageKey().replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      var match = document.cookie.match(new RegExp('(?:^|;\\s*)' + escapedKey + '=([^;]*)'));
      return match ? decodeURIComponent(match[1]) : null;
    } catch (e) {
      return null;
    }
  }

  function writeCookie() {
    try {
      document.cookie = encodeURIComponent(storageKey()) + '=1; path=/; max-age=' + COOKIE_MAX_AGE + '; SameSite=Lax';
    } catch (e) {}
  }

  function isDismissed() {
    return dismissedInMemory || readStorage() === '1' || readCookie() === '1';
  }

  function rememberDismissal() {
    dismissedInMemory = true;
    writeStorage();
    writeCookie();
  }

  function isOpen() {
    return !!modal && !modal.classList.contains('hidden');
  }

  function setOpen(open) {
    if (!modal) return;
    if (open) {
      modal.classList.remove('hidden');
      modal.setAttribute('aria-hidden', 'false');
      if (document.body && document.body.classList) {
        document.body.classList.add('release-note-open');
      }
    } else {
      modal.classList.add('hidden');
      modal.setAttribute('aria-hidden', 'true');
      if (document.body && document.body.classList) {
        document.body.classList.remove('release-note-open');
      }
    }
  }

  function focusableElements() {
    if (!modal || !modal.querySelectorAll) return [];
    return Array.prototype.slice.call(
      modal.querySelectorAll(
        'button:not([disabled]), a[href], input:not([disabled]), ' +
          'select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      )
    );
  }

  function dismiss() {
    if (!modal || !isOpen()) return;
    rememberDismissal();
    setOpen(false);
    if (previousFocus && typeof previousFocus.focus === 'function') {
      previousFocus.focus();
    }
    previousFocus = null;
  }

  function show() {
    if (!modal || isDismissed()) return;
    previousFocus = document.activeElement;
    setOpen(true);
    var focusables = focusableElements();
    if (focusables.length > 0 && typeof focusables[0].focus === 'function') {
      focusables[0].focus();
    }
  }

  function bindDismissButtons() {
    if (!modal || !modal.querySelectorAll || modal.dataset.releaseNoteBound === '1') return;
    modal.dataset.releaseNoteBound = '1';
    var buttons = modal.querySelectorAll('[data-release-note-dismiss]');
    for (var i = 0; i < buttons.length; i++) {
      buttons[i].addEventListener('click', dismiss);
    }
  }

  function initialize() {
    modal = findModal();
    if (!modal) return;
    bindDismissButtons();
    if (!isDismissed() && !isOpen()) show();
  }

  function handleKeydown(event) {
    if (!isOpen()) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      dismiss();
      return;
    }
    if (event.key !== 'Tab') return;

    var focusables = focusableElements();
    if (focusables.length === 0) return;
    var first = focusables[0];
    var last = focusables[focusables.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  document.addEventListener('keydown', handleKeydown);
  document.addEventListener('DOMContentLoaded', initialize);
  document.addEventListener('htmx:afterSwap', initialize);

  var controller = {
    dismiss: dismiss,
    isOpen: isOpen,
    show: show,
    storageKey: storageKey,
  };
  window.__afritaReleaseNoteController = controller;

  if (document.readyState !== 'loading') initialize();
  return controller;
});
