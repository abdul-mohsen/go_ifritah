/* smart-search.js — redesigned list-page search UX.
 *
 * Wraps every <input.search-input[name="q"]> on every list page with:
 *   • inline spinner + clear button (no more disabled submit during type)
 *   • instant client-side row-filter on #list-results <tbody> rows for 0-ms
 *     perceived latency while the HTMX server fetch is in flight
 *   • live "X of Y" match counter
 *   • recent-searches dropdown (top 5 per resource, localStorage)
 *   • removable filter chips for sibling <select>s in the same form
 *   • TYPED field-filter chips (GitHub/Linear/Stripe pattern): a "+ فلتر"
 *     button opens a popover of fields available for the current page
 *     (Phone, Sequence #, Tax #, VIN, Barcode, …). Picking one drops a
 *     removable chip and injects a hidden input into the form so the BE
 *     receives a typed param it can hit with a B-Tree index instead of a
 *     multi-column FULLTEXT scan.
 *   • Smart hint: when the user types digits-only into `q`, a "Tab to
 *     search by phone" suggestion promotes the value to a phone chip.
 *   • keyboard shortcuts: "/" and Ctrl/Cmd-K focus, Esc clears, Enter submits
 *
 * Purely additive: no changes to handlers, HTMX attributes, or templates
 * other than loading this file + smart-search.css from base.html.
 */
(function () {
  'use strict';

  var RECENT_KEY_PREFIX = 'afrita_smart_search_recent_';
  var RECENT_MAX = 5;
  var ACTIVE_CLASS = 'smart-hidden';

  // ── Per-resource typed-filter menus ───────────────────────────────
  // These mirror the ID-PREFIX columns in api_docs/search/SEARCH_API.md
  // §4 so the BE can hit indexed columns directly. The "param" is the
  // query-string key the backend will receive; "icon" is decorative.
  var FIELD_MENUS = {
    '/dashboard/invoices': [
      { param: 'sequence_number', label: 'رقم الفاتورة', icon: '#', kind: 'digits' },
      { param: 'phone', label: 'رقم الهاتف', icon: '☎', kind: 'digits' },
      { param: 'vin', label: 'رقم الهيكل', icon: '🚗', kind: 'alnum' },
    ],
    '/dashboard/purchase-bills': [
      { param: 'sequence_number', label: 'رقم الفاتورة', icon: '#', kind: 'digits' },
      { param: 'supplier_sequence_number', label: 'رقم فاتورة المورد', icon: '№', kind: 'alnum' },
    ],
    '/dashboard/products': [
      { param: 'part_number', label: 'رقم القطعة', icon: '⛓', kind: 'alnum' },
      { param: 'barcode', label: 'الباركود', icon: '|||', kind: 'alnum' },
    ],
    '/dashboard/clients': [
      { param: 'phone', label: 'رقم الهاتف', icon: '☎', kind: 'digits' },
      { param: 'vat_number', label: 'الرقم الضريبي', icon: '%', kind: 'alnum' },
      { param: 'commercial_registration', label: 'السجل التجاري', icon: '📄', kind: 'alnum' },
    ],
    '/dashboard/suppliers': [
      { param: 'phone', label: 'رقم الهاتف', icon: '☎', kind: 'digits' },
      { param: 'vat_number', label: 'الرقم الضريبي', icon: '%', kind: 'alnum' },
      { param: 'commercial_registration', label: 'السجل التجاري', icon: '📄', kind: 'alnum' },
    ],
    '/dashboard/orders': [
      { param: 'sequence_number', label: 'رقم الطلب', icon: '#', kind: 'digits' },
      { param: 'phone', label: 'رقم الهاتف', icon: '☎', kind: 'digits' },
      { param: 'vin', label: 'رقم الهيكل', icon: '🚗', kind: 'alnum' },
    ],
    '/dashboard/cash-vouchers': [
      { param: 'sequence_number', label: 'رقم السند', icon: '#', kind: 'digits' },
    ],
    '/dashboard/branches': [
      { param: 'phone', label: 'رقم الهاتف', icon: '☎', kind: 'digits' },
    ],
    '/dashboard/users': [
      { param: 'email', label: 'البريد الإلكتروني', icon: '@', kind: 'email' },
      { param: 'phone', label: 'رقم الهاتف', icon: '☎', kind: 'digits' },
    ],
  };

  function fieldMenuFor(form) {
    var path = location.pathname.replace(/\/$/, '');
    return FIELD_MENUS[path] || [];
  }

  // ── Arabic/Latin/digit normaliser ─────────────────────────────────
  function normalize(s) {
    if (s == null) return '';
    s = String(s).toLowerCase().trim();
    if (!s) return '';
    s = s.replace(/[\u064B-\u0652\u0640]/g, '');                 // diacritics + tatweel
    s = s.replace(/[\u0623\u0625\u0622]/g, '\u0627');            // أإآ → ا
    s = s.replace(/\u0629/g, '\u0647');                          // ة → ه
    s = s.replace(/\u0649/g, '\u064A');                          // ى → ي
    s = s.replace(/[\u0660-\u0669]/g, function (d) {             // arabic-indic
      return String.fromCharCode(d.charCodeAt(0) - 0x0660 + 0x30);
    });
    s = s.replace(/[\u06F0-\u06F9]/g, function (d) {             // ext arabic-indic
      return String.fromCharCode(d.charCodeAt(0) - 0x06F0 + 0x30);
    });
    return s.replace(/\s+/g, ' ');
  }

  function tokensMatch(query, haystack) {
    var q = normalize(query);
    if (!q) return true;
    var h = normalize(haystack);
    var toks = q.split(' ');
    for (var i = 0; i < toks.length; i++) {
      if (toks[i] && h.indexOf(toks[i]) === -1) return false;
    }
    return true;
  }

  // ── Recent-search storage ─────────────────────────────────────────
  function recentKey() {
    return RECENT_KEY_PREFIX + (location.pathname || 'global');
  }
  function loadRecent() {
    try {
      var raw = localStorage.getItem(recentKey());
      var arr = raw ? JSON.parse(raw) : [];
      return Array.isArray(arr) ? arr.slice(0, RECENT_MAX) : [];
    } catch (e) { return []; }
  }
  function saveRecent(term) {
    term = String(term || '').trim();
    if (!term) return;
    try {
      var list = loadRecent().filter(function (t) { return t !== term; });
      list.unshift(term);
      list = list.slice(0, RECENT_MAX);
      localStorage.setItem(recentKey(), JSON.stringify(list));
    } catch (e) { /* quota / private mode: ignore */ }
  }

  // ── DOM helpers ───────────────────────────────────────────────────
  function el(tag, attrs, children) {
    var n = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function (k) {
        if (k === 'class') n.className = attrs[k];
        else if (k === 'text') n.textContent = attrs[k];
        else if (k === 'html') n.innerHTML = attrs[k];
        else n.setAttribute(k, attrs[k]);
      });
    }
    (children || []).forEach(function (c) { if (c) n.appendChild(c); });
    return n;
  }

  // SVG icon factories (kept tiny inline to avoid extra requests)
  var ICON_SEARCH =
    '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
    '<circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>';
  var ICON_X =
    '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
    '<path d="M18 6L6 18M6 6l12 12"/></svg>';
  var ICON_SPIN =
    '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" aria-hidden="true">' +
    '<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>' +
    '<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>';

  // ── Row-level instant filter ──────────────────────────────────────
  function instantFilterRows(form, term) {
    // Find table tbody inside #list-results within the same page region.
    var results = document.getElementById('list-results') || form.parentElement;
    if (!results) return { matched: 0, total: 0 };
    var rows = results.querySelectorAll('tbody > tr');
    var matched = 0, total = 0;
    rows.forEach(function (row) {
      // Skip empty-state rows
      if (row.classList.contains('empty-row') || row.dataset.emptyRow === '1') return;
      // Skip rows that span the whole table (placeholder)
      if (row.cells && row.cells.length === 1 && row.cells[0].colSpan > 1) return;
      total++;
      var visibleText = row.textContent || '';
      if (tokensMatch(term, visibleText)) {
        row.classList.remove(ACTIVE_CLASS);
        matched++;
      } else {
        row.classList.add(ACTIVE_CLASS);
      }
    });
    return { matched: matched, total: total };
  }

  // ── Filter-chip rendering for sibling <select>s ───────────────────
  function selectActiveChip(select) {
    var v = select.value;
    if (v === '' || v == null) return null;
    var selOpt = select.options[select.selectedIndex];
    if (!selOpt) return null;
    var label = (selOpt.textContent || '').trim();
    if (!label) return null;
    // Skip per-page (we surface that separately via toolbar)
    if (select.name === 'per' || select.classList.contains('per-page-select')) return null;
    var chip = el('button', {
      type: 'button',
      class: 'smart-chip',
      'data-name': select.name,
      title: label,
      'aria-label': label + ' — ' + (window.__L_remove || 'إزالة')
    });
    chip.appendChild(document.createTextNode(label));
    var x = el('span', { class: 'smart-chip-x', html: ICON_X });
    chip.appendChild(x);
    chip.addEventListener('click', function (e) {
      e.preventDefault();
      // Reset to first (empty) option then submit
      select.value = '';
      // dispatch a change event for HTMX form trigger
      select.dispatchEvent(new Event('change', { bubbles: true }));
      if (typeof select.form.requestSubmit === 'function') {
        select.form.requestSubmit();
      } else {
        select.form.submit();
      }
    });
    return chip;
  }

  function renderChips(form, container) {
    container.innerHTML = '';
    var selects = form.querySelectorAll('select');
    var any = false;
    selects.forEach(function (sel) {
      var chip = selectActiveChip(sel);
      if (chip) { container.appendChild(chip); any = true; }
    });
    // Typed-field chips (hidden inputs the user added via the "+ فلتر" menu)
    var typed = form.querySelectorAll('input[type="hidden"][data-smart-typed="1"]');
    typed.forEach(function (hi) {
      var menu = fieldMenuFor(form);
      var def = menu.find ? menu.find(function (f) { return f.param === hi.name; })
        : null;
      var label = def ? def.label : hi.name;
      var icon = def ? def.icon : '⌗';
      var chip = el('button', {
        type: 'button',
        class: 'smart-chip smart-chip-typed',
        title: label + ': ' + hi.value,
      });
      chip.appendChild(document.createTextNode(icon + '  ' + label + ': ' + hi.value));
      var x = el('span', { class: 'smart-chip-x', html: ICON_X });
      chip.appendChild(x);
      chip.addEventListener('click', function (e) {
        e.preventDefault();
        hi.parentNode.removeChild(hi);
        if (typeof form.requestSubmit === 'function') form.requestSubmit();
      });
      container.appendChild(chip);
      any = true;
    });
    container.style.display = any ? '' : 'none';
  }

  // ── Typed-filter popover ──────────────────────────────────────────
  function buildFilterPopover(form, anchorBtn, input) {
    var menu = fieldMenuFor(form);
    if (!menu.length) return null;
    var pop = el('div', { class: 'smart-filter-pop', role: 'menu' });
    var head = el('div', { class: 'smart-recent-head', text: 'بحث في حقل محدد' });
    pop.appendChild(head);
    menu.forEach(function (f) {
      // Skip if a chip for this field already exists
      if (form.querySelector('input[type="hidden"][data-smart-typed="1"][name="' + f.param + '"]')) {
        return;
      }
      var item = el('button', { type: 'button', class: 'smart-filter-pop-item' });
      item.appendChild(document.createTextNode(f.icon + '  ' + f.label));
      item.addEventListener('mousedown', function (e) {
        e.preventDefault();
        promptForValue(form, f, input);
        pop.remove();
      });
      pop.appendChild(item);
    });
    if (!pop.querySelector('.smart-filter-pop-item')) {
      var none = el('div', { class: 'smart-filter-pop-empty', text: 'كل الحقول مضافة' });
      pop.appendChild(none);
    }
    return pop;
  }

  function promptForValue(form, fieldDef, input) {
    // Two paths: if the q input already has a value matching the kind, just
    // promote it (no second click). Otherwise open an inline mini-input.
    var current = input.value.trim();
    var matches =
      (fieldDef.kind === 'digits' && /^[\u0660-\u0669\u06F0-\u06F90-9]+$/.test(current)) ||
      (fieldDef.kind === 'alnum' && current && !/\s/.test(current)) ||
      (fieldDef.kind === 'email' && /@/.test(current));
    if (matches) {
      addTypedFilter(form, fieldDef.param, current, fieldDef.kind);
      input.value = '';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      if (typeof form.requestSubmit === 'function') form.requestSubmit();
      return;
    }
    // Inline mini-prompt — replace the search input contents with a
    // placeholder hint and listen for every keystroke (live update).
    var prevPh = input.placeholder;
    input.placeholder = fieldDef.label + ' — اكتب القيمة (البحث مباشر)';
    input.value = '';
    input.focus();
    function onKey(e) {
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        var v = input.value.trim();
        cleanup();
        if (v) {
          addTypedFilter(form, fieldDef.param, v, fieldDef.kind);
          input.value = '';
          input.dispatchEvent(new Event('input', { bubbles: true }));
          if (typeof form.requestSubmit === 'function') form.requestSubmit();
        }
      } else if (e.key === 'Escape') {
        e.preventDefault();
        cleanup();
      }
    }
    var submitTimer = null;
    function onInput(e) {
      var v = input.value.trim();
      if (v) {
        addTypedFilter(form, fieldDef.param, v, fieldDef.kind);
        input.dispatchEvent(new Event('input', { bubbles: true }));
        // Debounce form submission: cancel previous timer, set new one
        if (submitTimer) clearTimeout(submitTimer);
        submitTimer = setTimeout(function () {
          if (typeof form.requestSubmit === 'function') form.requestSubmit();
        }, 350);  // 350ms debounce
      }
    }
    function cleanup() {
      input.placeholder = prevPh;
      input.removeEventListener('keydown', onKey);
      input.removeEventListener('input', onInput);
      if (submitTimer) clearTimeout(submitTimer);
    }
    input.addEventListener('keydown', onKey);
    input.addEventListener('input', onInput);
  }

  // Backend (mailbox #34) does plain LIKE on the raw column — no Arabic-Indic
  // folding, no non-digit stripping. Normalise on the FE so bookmarked URLs
  // with pretty values like "+966 50 1234567" or "٠٥٠١٢٣٤٥٦٧" still match.
  function sanitizeForKind(value, kind) {
    var v = (value == null ? '' : String(value));
    if (kind !== 'digits') return v.trim();
    // Fold Arabic-Indic + extended Arabic-Indic digits to ASCII.
    v = v.replace(/[\u0660-\u0669]/g, function (d) {
      return String.fromCharCode(d.charCodeAt(0) - 0x0660 + 0x30);
    });
    v = v.replace(/[\u06F0-\u06F9]/g, function (d) {
      return String.fromCharCode(d.charCodeAt(0) - 0x06F0 + 0x30);
    });
    // Strip everything that isn't an ASCII digit.
    return v.replace(/\D+/g, '');
  }

  function addTypedFilter(form, name, value, kind) {
    var clean = sanitizeForKind(value, kind);
    if (!clean) return; // empty after normalisation = no-op
    // De-dupe: if a hidden of this name already exists, replace it.
    var existing = form.querySelector('input[type="hidden"][data-smart-typed="1"][name="' + name + '"]');
    if (existing) existing.parentNode.removeChild(existing);
    var hi = el('input', {
      type: 'hidden',
      name: name,
      value: clean,
      'data-smart-typed': '1',
    });
    form.appendChild(hi);
  }

  // ── Recents dropdown ──────────────────────────────────────────────
  function renderRecents(panel, input) {
    var items = loadRecent();
    panel.innerHTML = '';
    if (!items.length) { panel.style.display = 'none'; return; }
    var head = el('div', { class: 'smart-recent-head', text: 'عمليات بحث سابقة' });
    panel.appendChild(head);
    items.forEach(function (term) {
      var b = el('button', { type: 'button', class: 'smart-recent-item', text: term });
      b.addEventListener('mousedown', function (e) {
        e.preventDefault();
        input.value = term;
        input.dispatchEvent(new Event('input', { bubbles: true }));
        if (typeof input.form.requestSubmit === 'function') {
          input.form.requestSubmit();
        }
        panel.style.display = 'none';
      });
      panel.appendChild(b);
    });
    panel.style.display = '';
  }

  // ── Wrap a single input.search-input[name="q"] ────────────────────
  function enhance(input) {
    if (input.dataset._smartSearchBound === '1') return;
    input.dataset._smartSearchBound = '1';
    var form = input.form;
    if (!form) return;

    // Build wrapper preserving the input's place in the DOM
    var wrap = el('div', { class: 'smart-search-wrap' });
    input.parentNode.insertBefore(wrap, input);
    wrap.appendChild(input);

    // Decorative leading magnifier
    var leadIcon = el('span', { class: 'smart-search-icon', html: ICON_SEARCH });
    wrap.insertBefore(leadIcon, input);

    // Trailing controls (spinner + clear)
    var spinner = el('span', { class: 'smart-search-spin', html: ICON_SPIN });
    spinner.style.display = 'none';
    wrap.appendChild(spinner);
    var clearBtn = el('button', {
      type: 'button',
      class: 'smart-search-clear',
      'aria-label': 'مسح',
      title: 'مسح',
      html: ICON_X
    });
    clearBtn.style.display = input.value ? '' : 'none';
    wrap.appendChild(clearBtn);

    // Recents dropdown
    var recents = el('div', { class: 'smart-recents' });
    recents.style.display = 'none';
    wrap.appendChild(recents);

    // "+ فلتر" button (typed-field menu) — only if this page has fields
    var menu = fieldMenuFor(form);
    var filterBtn = null;
    if (menu.length) {
      filterBtn = el('button', {
        type: 'button',
        class: 'smart-filter-btn',
        'aria-label': 'إضافة فلتر',
        title: 'بحث في حقل محدد',
      });
      filterBtn.appendChild(document.createTextNode('+ فلتر'));
      // Place AFTER the wrap inside the form so it sits beside the input
      wrap.parentNode.insertBefore(filterBtn, wrap.nextSibling);

      filterBtn.addEventListener('click', function (e) {
        e.preventDefault();
        // Toggle: if a popover is already open, close it.
        var existing = document.querySelector('.smart-filter-pop');
        if (existing) { existing.remove(); return; }
        var pop = buildFilterPopover(form, filterBtn, input);
        if (!pop) return;
        // Position relative to the button
        filterBtn.parentNode.appendChild(pop);
        pop.style.position = 'absolute';
        var r = filterBtn.getBoundingClientRect();
        var pr = filterBtn.parentNode.getBoundingClientRect();
        pop.style.top = (r.bottom - pr.top + 4) + 'px';
        pop.style.insetInlineStart = (r.left - pr.left) + 'px';
        // Close on outside click
        setTimeout(function () {
          document.addEventListener('mousedown', function close(ev) {
            if (!pop.contains(ev.target) && ev.target !== filterBtn) {
              pop.remove();
              document.removeEventListener('mousedown', close);
            }
          });
        }, 0);
      });
    }

    // Smart hint (digits-only typed → suggest typed phone/seq filter)
    var hint = el('div', { class: 'smart-search-hint' });
    hint.style.display = 'none';
    wrap.appendChild(hint);

    function refreshHint() {
      hint.innerHTML = '';
      var v = input.value.trim();
      if (!v || !menu.length) { hint.style.display = 'none'; return; }
      var isDigits = /^[\u0660-\u0669\u06F0-\u06F90-9\s]+$/.test(v);
      // Build the list of fields whose `kind` matches what the user typed.
      // For digits we may have BOTH phone and sequence_number on the same
      // page (e.g. invoices); offer both so the user can disambiguate.
      var picks = [];
      if (isDigits) {
        var phoneF = menu.find(function (f) { return f.param === 'phone'; });
        var seqF = menu.find(function (f) { return f.param === 'sequence_number'; });
        if (phoneF) picks.push(phoneF);
        if (seqF) picks.push(seqF);
      } else if (/@/.test(v)) {
        var emailF = menu.find(function (f) { return f.param === 'email'; });
        if (emailF) picks.push(emailF);
      }
      if (!picks.length) { hint.style.display = 'none'; return; }
      hint.style.display = '';
      picks.forEach(function (pickField, idx) {
        var btn = el('button', { type: 'button', class: 'smart-search-hint-btn' });
        // First pick on Tab; second pick on Shift+Tab (only when there are 2)
        var shortcut = (picks.length === 1 || idx === 0) ? 'Tab' : 'Shift+Tab';
        btn.appendChild(document.createTextNode(shortcut + ' — بحث بـ ' + pickField.label + ': ' + v));
        btn.dataset.pickIdx = String(idx);
        btn.addEventListener('mousedown', function (e) {
          e.preventDefault();
          addTypedFilter(form, pickField.param, v, pickField.kind);
          input.value = '';
          input.dispatchEvent(new Event('input', { bubbles: true }));
          if (typeof form.requestSubmit === 'function') form.requestSubmit();
        });
        hint.appendChild(btn);
      });
    }

    // Below: chips row + counter (rendered in toolbar after the form)
    var meta = el('div', { class: 'smart-search-meta' });
    var chips = el('div', { class: 'smart-chips' });
    var counter = el('span', { class: 'smart-counter' });
    meta.appendChild(chips);
    meta.appendChild(counter);
    // Place meta as a sibling AFTER the form within .list-toolbar so it
    // stretches across the toolbar without breaking the existing flex row.
    var toolbar = form.closest('.list-toolbar');
    if (toolbar) {
      toolbar.parentNode.insertBefore(meta, toolbar.nextSibling);
    } else {
      form.parentNode.insertBefore(meta, form.nextSibling);
    }

    // Add CSS hook so we can style the legacy submit button to be unobtrusive
    form.classList.add('smart-search-form');

    // ── Event wiring ────────────────────────────────────────────────
    function updateClearVisibility() {
      clearBtn.style.display = input.value ? '' : 'none';
    }

    function applyInstant() {
      var stats = instantFilterRows(form, input.value);
      if (input.value) {
        counter.textContent = stats.matched + ' من ' + stats.total;
        counter.style.display = '';
      } else {
        counter.textContent = '';
        counter.style.display = 'none';
      }
    }

    input.addEventListener('input', function () {
      updateClearVisibility();
      applyInstant();
      refreshHint();
    });

    input.addEventListener('focus', function () {
      if (!input.value) renderRecents(recents, input);
    });
    input.addEventListener('blur', function () {
      // Delay so click on a recent item lands first
      setTimeout(function () { recents.style.display = 'none'; }, 150);
    });

    input.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && input.value) {
        e.preventDefault();
        input.value = '';
        updateClearVisibility();
        applyInstant();
        refreshHint();
        if (typeof form.requestSubmit === 'function') form.requestSubmit();
      } else if (e.key === 'Tab' && hint.style.display !== 'none') {
        // Tab → first pick (phone preferred); Shift+Tab → second pick if any (seq).
        var idx = e.shiftKey ? 1 : 0;
        var btn = hint.querySelector('button[data-pick-idx="' + idx + '"]') ||
          (idx === 0 ? hint.querySelector('button') : null);
        if (btn) {
          e.preventDefault();
          btn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
        }
      }
    });

    clearBtn.addEventListener('click', function (e) {
      e.preventDefault();
      input.value = '';
      input.focus();
      updateClearVisibility();
      applyInstant();
      if (typeof form.requestSubmit === 'function') form.requestSubmit();
    });

    // Spinner reflects HTMX request lifecycle for THIS form
    form.addEventListener('htmx:beforeRequest', function () { spinner.style.display = ''; });
    form.addEventListener('htmx:afterRequest', function () { spinner.style.display = 'none'; });
    form.addEventListener('htmx:sendError', function () { spinner.style.display = 'none'; });

    // Persist on actual server submit (what the user explicitly searched)
    form.addEventListener('submit', function () {
      saveRecent(input.value);
    });
    // Also persist when HTMX completes a request with a non-empty query
    // (the auto-debounced trigger doesn't fire `submit`).
    form.addEventListener('htmx:afterRequest', function () {
      if (input.value) saveRecent(input.value);
    });

    // Re-render chips whenever any select changes (HTMX rewrites the form on
    // swap, so we re-render on swap too).
    form.querySelectorAll('select').forEach(function (sel) {
      sel.addEventListener('change', function () { renderChips(form, chips); });
    });
    document.addEventListener('htmx:afterSwap', function () { renderChips(form, chips); });

    // Rehydrate typed chips from the URL: any ?phone=050&tax_number=… that
    // matches a known field for this page becomes a hidden input + chip so
    // the user lands on the same filtered view they bookmarked / refreshed.
    (function () {
      var qs = new URLSearchParams(location.search);
      menu.forEach(function (f) {
        var v = qs.get(f.param);
        if (v) addTypedFilter(form, f.param, v, f.kind);
      });
    })();

    renderChips(form, chips);

    // Initial pass (in case the page loads with a pre-filled query)
    if (input.value) applyInstant();
  }

  // ── Global keyboard shortcut: "/" focuses first smart-search input ─
  document.addEventListener('keydown', function (e) {
    // Don't hijack typing in inputs/contenteditable
    var t = e.target;
    var inField = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || (t.isContentEditable === true));
    if (e.key === '/' && !inField && !e.ctrlKey && !e.metaKey && !e.altKey) {
      var first = document.querySelector('input.search-input[name="q"]');
      if (first) {
        e.preventDefault();
        first.focus();
        first.select();
      }
    }
  });

  function initAll() {
    document.querySelectorAll('input.search-input[name="q"]').forEach(enhance);
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initAll);
  } else {
    initAll();
  }
  // Re-init after HTMX swaps that might bring in new search forms
  document.addEventListener('htmx:afterSwap', initAll);
})();
