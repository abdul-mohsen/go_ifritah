// ── Toast Notification System ──────────────────────────────
(function () {
    let TOAST_DURATION = 5000;
    let lastToast = { message: '', type: '', at: 0 };
    let ICONS = {
        error: '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z"/></svg>',
        success: '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>',
        warning: '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M10.29 3.86l-8.6 14.86A1 1 0 002.56 20h18.88a1 1 0 00.87-1.28l-8.6-14.86a1 1 0 00-1.72 0z"/></svg>',
        info: '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4m0-4h.01"/></svg>'
    };

    window.showToast = function (message, type) {
        type = type || 'error';
        if (!message) message = t('generic_error');

        let now = Date.now();
        if (message === lastToast.message && type === lastToast.type && now - lastToast.at < 750) return;
        lastToast = { message: message, type: type, at: now };

        let container = document.getElementById('toast-container');
        if (!container) return;

        let toast = document.createElement('div');
        toast.className = 'toast toast-' + type;
        toast.innerHTML = (ICONS[type] || ICONS.error) +
            '<span>' + message + '</span>' +
            '<button class="toast-close">&times;</button>';

        toast.querySelector('.toast-close').onclick = function () {
            toast.classList.add('toast-out');
            setTimeout(function () { toast.remove(); }, 300);
        };

        container.appendChild(toast);

        // Auto-remove
        let ref = toast;
        setTimeout(function () {
            if (ref.parentElement) {
                ref.classList.add('toast-out');
                setTimeout(function () { ref.remove(); }, 300);
            }
        }, TOAST_DURATION);
    };
})();

// ── PDF Opener (fetches PDF via JS, shows toast on error) ─────────
(function () {
    function handlePDF(url, action) {
        // Pre-open the window synchronously (during the click event) so popup
        // blockers don't suppress it. We navigate or close it after fetch.
        let win = null;
        if (action === 'open') {
            win = window.open('about:blank', '_blank');
        }

        fetch(url, { credentials: 'same-origin' })
            .then(function (resp) {
                let ct = resp.headers.get('content-type') || '';
                if (resp.ok && ct.indexOf('application/pdf') !== -1) {
                    return resp.blob().then(function (blob) {
                        let blobUrl = URL.createObjectURL(blob);
                        if (action === 'print') {
                            let iframe = document.createElement('iframe');
                            iframe.style.position = 'fixed';
                            iframe.style.right = '-9999px';
                            iframe.style.width = '0';
                            iframe.style.height = '0';
                            iframe.src = blobUrl;
                            document.body.appendChild(iframe);
                            iframe.onload = function () {
                                try {
                                    iframe.contentWindow.focus();
                                    iframe.contentWindow.print();
                                } catch (e) { }
                                // Use afterprint event to clean up after user finishes
                                // Falls back to a long timeout if afterprint isn't supported
                                let cleaned = false;
                                function cleanup() {
                                    if (cleaned) return;
                                    cleaned = true;
                                    try { document.body.removeChild(iframe); } catch (e) { }
                                    URL.revokeObjectURL(blobUrl);
                                }
                                try {
                                    iframe.contentWindow.addEventListener('afterprint', cleanup);
                                } catch (e) { }
                                // Fallback: clean up after 5 minutes if afterprint never fires
                                setTimeout(cleanup, 300000);
                            };
                        } else if (win) {
                            win.location.href = blobUrl;
                            // Revoke after a delay so the new tab can render
                            setTimeout(function () { URL.revokeObjectURL(blobUrl); }, 120000);
                        }
                    });
                }
                // Error response — close pre-opened window and show toast
                if (win) { try { win.close(); } catch (e) { } }
                return resp.text().then(function (text) {
                    let msg = t('pdf_load_failed');
                    try {
                        let json = JSON.parse(text);
                        if (json.message) msg = json.message;
                    } catch (e) { }
                    window.showToast(msg, 'error');
                });
            })
            .catch(function () {
                if (win) { try { win.close(); } catch (e) { } }
                window.showToast(t('server_unreachable'), 'error');
            });
    }

    window.openPDF = function (url) { handlePDF(url, 'open'); };
    window.printPDF = function (url) { handlePDF(url, 'print'); };
})();

// ── Flash Cookie Reader (shows toast from redirected pages) ───────
(function () {
    let match = document.cookie.match(/(?:^|;\s*)afrita_flash=([^;]*)/);
    if (match) {
        try {
            let flash = JSON.parse(decodeURIComponent(match[1]));
            if (flash.message && window.showToast) {
                // Small delay to ensure DOM is ready
                setTimeout(function () {
                    window.showToast(flash.message, flash.type || 'success');
                }, 100);
            }
        } catch (e) { }
        // Clear the cookie
        document.cookie = 'afrita_flash=; path=/; max-age=0';
    }
})();

// ── Global Loading Overlay ────────────────────────────────────
(function () {
    let loadingEl = document.getElementById('global-loading');
    let showTimer = null;

    window.__showLoading = function () {
        // Debounce: only show after 200ms to avoid flicker on fast requests
        if (showTimer) return;
        showTimer = setTimeout(function () {
            if (loadingEl) {
                loadingEl.style.display = '';
                loadingEl.classList.remove('hidden');
            }
        }, 200);
    };

    window.__hideLoading = function () {
        if (showTimer) {
            clearTimeout(showTimer);
            showTimer = null;
        }
        if (loadingEl) {
            loadingEl.classList.add('hidden');
            loadingEl.style.display = 'none';
        }
    };
})();

// ── Disable Submit Buttons During HTMX Requests ──────────────
(function () {
    let disabledButtons = [];

    document.addEventListener("htmx:beforeRequest", function (evt) {
        // Show global loading
        if (window.__showLoading) window.__showLoading();

        // Disable all submit buttons in the requesting form
        let form = evt.detail.elt;
        if (form && form.tagName !== 'FORM') form = form.closest('form');
        if (form) {
            let btns = form.querySelectorAll('button[type="submit"], button:not([type])');
            btns.forEach(function (btn) {
                btn.disabled = true;
                btn.classList.add('htmx-requesting');
            });
            disabledButtons = disabledButtons.concat(Array.from(btns));
        }
    });

    function reEnableButtons() {
        if (window.__hideLoading) window.__hideLoading();
        disabledButtons.forEach(function (btn) {
            btn.disabled = false;
            btn.classList.remove('htmx-requesting');
        });
        disabledButtons = [];
    }

    document.addEventListener("htmx:afterRequest", reEnableButtons);
    document.addEventListener("htmx:responseError", reEnableButtons);
    document.addEventListener("htmx:sendError", reEnableButtons);
})();

// HTMX helpers
document.addEventListener("htmx:confirm", function (evt) {
    // Only confirm deletions
    let verb = evt.detail.requestConfig ? evt.detail.requestConfig.verb : '';
    let path = evt.detail.path || '';
    if (verb === 'delete' || path.indexOf('/delete') !== -1) {
        if (!confirm(t('confirm_delete'))) {
            evt.preventDefault();
        }
    }
});

// Intercept non-2xx HTMX responses — show toast instead of swapping raw error text
document.addEventListener("htmx:beforeSwap", function (evt) {
    let xhr = evt.detail.xhr;
    if (xhr && xhr.status >= 400) {
        evt.detail.shouldSwap = false;
        if (window.__hideLoading) window.__hideLoading();

        // Extract message from response
        let msg = '';
        let text = (xhr.responseText || '').trim();
        if (text) {
            // Try JSON: {"detail":"..."} or {"error":"..."} or {"message":"..."}
            try {
                let json = JSON.parse(text);
                msg = json.detail || json.error || json.message || json.msg || '';
            } catch (e) {
                // Use plain text if short and not HTML
                if (text.length < 200 && text.charAt(0) !== '<') {
                    msg = text;
                }
            }
        }

        // Provide meaningful fallback based on status code
        if (!msg) {
            let statusMessages = {
                400: t('http_400'),
                401: t('http_401'),
                403: t('http_403'),
                404: t('http_404'),
                409: t('http_409'),
                422: t('http_422'),
                429: t('http_429'),
                500: t('http_500'),
                502: t('http_502'),
                503: t('http_503')
            };
            msg = statusMessages[xhr.status] || t('generic_error');
        }

        window.showToast(msg, 'error');

        // Auto-redirect to login on 401
        if (xhr.status === 401) {
            setTimeout(function () { window.location.href = '/login'; }, 2000);
        }
    }
});

// Handle network errors (backend unreachable)
document.addEventListener("htmx:responseError", function () {
    if (window.__hideLoading) window.__hideLoading();
    window.showToast(t('network_check_error'), 'error');
});

// Handle request send failures (timeout, connection refused)
document.addEventListener("htmx:sendError", function () {
    if (window.__hideLoading) window.__hideLoading();
    window.showToast(t('request_failed'), 'error');
});

// Listen for HX-Trigger showToast events (sent by backend)
document.addEventListener("showToast", function (evt) {
    let d = evt.detail || {};
    window.showToast(d.message || '', d.type || 'error');
});

// ── Reset accumulated query string on HTMX GET form submits ───────
// Without this, list pages that use `hx-get=""` reuse window.location
// (which already contains ?q=a&sort=…) so the next typed value gets
// appended (?q=a&sort=…&q=b).
//
// Strategy: for list-search forms (the `hx-target="#list-results"`
// convention used by every list page) the FORM IS THE SOURCE OF TRUTH —
// any param that isn't on the form right now should be gone from the
// URL too. Otherwise removing a typed-filter chip leaves its `phone=…`
// glued to every subsequent submit forever (regression bug 2026-05-07).
//
// For non-list forms we keep the old conservative behaviour: drop only
// keys the form is re-submitting so deep-link state on bespoke pages
// survives.
function __isListSearchForm(el) {
    if (!el) return false;
    const form = el.tagName === 'FORM' ? el : (el.closest && el.closest('form'));
    if (!form) return false;
    return form.getAttribute('hx-target') === '#list-results';
}

document.addEventListener("htmx:configRequest", function (evt) {
    let d = evt.detail;
    if (!d) return;
    if (d.verb === 'get' && d.path && d.parameters && typeof d.parameters === 'object') {
        let qIdx = d.path.indexOf('?');
        if (qIdx >= 0) {
            let head = d.path.slice(0, qIdx);
            if (__isListSearchForm(evt.target || d.elt)) {
                // List-search form owns the URL — drop everything, the form
                // will rebuild the query from its current state.
                d.path = head;
            } else {
                // Non-list form: only strip keys the form is re-adding so
                // unrelated deep-link state is preserved.
                let qs = new URLSearchParams(d.path.slice(qIdx + 1));
                Object.keys(d.parameters).forEach(function (k) { qs.delete(k); });
                let rest = qs.toString();
                d.path = rest ? head + '?' + rest : head;
            }
        }
    }
    if (d.parameters && typeof d.parameters === 'object') {
        Object.keys(d.parameters).forEach(function (k) {
            let v = d.parameters[k];
            if (v === '' || v == null) delete d.parameters[k];
        });
    }
});

// ── Surface server-side error-alert from swapped responses ────────
// List pages render `{{ template "error-alert" . }}` OUTSIDE the
// `#list-results` swap target, so when hx-select drops everything but
// the table, an .error returned from the server is invisible. Pull any
// [role="alert"] out of the response and show it as a toast.
document.addEventListener("htmx:beforeSwap", function (evt) {
    try {
        let xhr = evt.detail.xhr;
        if (!xhr || !xhr.response) return;
        let ct = xhr.getResponseHeader('content-type') || '';
        if (ct.indexOf('text/html') === -1) return;
        let doc = new DOMParser().parseFromString(xhr.response, 'text/html');
        let alerts = doc.querySelectorAll('[role="alert"]');
        if (!alerts.length) return;
        // De-dupe identical messages within a single swap
        let seen = {};
        alerts.forEach(function (a) {
            let msg = (a.textContent || '').trim();
            if (!msg || seen[msg]) return;
            seen[msg] = true;
            // Heuristic: red/error styling → error toast; otherwise info.
            let bg = a.getAttribute('style') || '';
            let type = /error|red|danger/i.test(bg + ' ' + a.className) ? 'error' : 'info';
            window.showToast(msg, type);
        });
    } catch (e) { }
});

// Hide loading indicator + auto-hide alerts after swap
document.addEventListener("htmx:afterSwap", function (evt) {
    if (window.__hideLoading) window.__hideLoading();
    // Auto-hide alerts that arrived with this swap, scoped to the swap target
    // so we don't trample the toast lifecycle (toasts are 3s/5s managed by
    // showToast() and live OUTSIDE the swap target).
    let target = (evt && evt.detail && evt.detail.target) || document;
    let alerts = target.querySelectorAll ? target.querySelectorAll("[role='alert']") : [];
    alerts.forEach(function (alert) {
        // Don't auto-hide toast container children (showToast manages those).
        if (alert.closest && alert.closest('#toast-container')) return;
        setTimeout(function () {
            alert.style.transition = 'opacity 0.3s ease';
            alert.style.opacity = '0';
            setTimeout(function () { alert.remove(); }, 300);
        }, 3000);
    });
});

// ── Keyboard Shortcuts ────────────────────────────────────────
(function () {
    document.addEventListener('keydown', function (e) {
        // Ctrl+K or Cmd+K → focus search input
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            let searchInput = document.querySelector('input[name="q"]');
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }
        // Escape → close open modals
        if (e.key === 'Escape') {
            let modals = document.querySelectorAll('.fixed:not(.hidden)');
            modals.forEach(function (modal) {
                if (modal.id && modal.id !== 'global-loading' && modal.id !== 'sidebar-overlay') {
                    modal.classList.add('hidden');
                }
            });
        }
    });
})();

// ── Delete Success Toast ──────────────────────────────────────
(function () {
    document.addEventListener('htmx:afterRequest', function (evt) {
        let xhr = evt.detail.xhr;
        let path = evt.detail.pathInfo ? evt.detail.pathInfo.requestPath : '';
        let verb = (evt.detail.requestConfig && evt.detail.requestConfig.verb) || '';
        if (xhr && xhr.status >= 200 && xhr.status < 300 &&
            (verb === 'delete' || path.indexOf('/delete') !== -1)) {
            window.showToast(t('delete_success'), 'success');
        }
    });
})();

// ── Search Form Loading State ──────────────────────────────────
// Disabled: smart-search.js renders an inline spinner inside the input
// and never disables the input. Replacing the submit button's HTML on
// every keystroke caused perceived lag; that behaviour is gone.
(function () { /* no-op (kept for diff history) */ })();

// ── Live debounced search on list pages ───────────────────────
// HTMX is the primary mechanism: list-page <form> elements declare
// hx-trigger="submit, input changed delay:400ms from:input[name='q'],
// change from:select" and hx-select="#list-results", so the table region
// is swapped without a full page reload.
//
// This block is the *fallback* for forms that don't have HTMX wired in
// (legacy/non-list pages). It auto-submits the form 400ms after the user
// stops typing in any input[name="q"] and resets page=0. ESC clears.
// Skipped entirely if the form already has hx-get/hx-post/hx-boost.
(function () {
    let DEBOUNCE_MS = 400;

    function findQInputs() {
        return document.querySelectorAll('input[name="q"]');
    }

    function isHtmxControlled(form) {
        if (!form) return false;
        return form.hasAttribute('hx-get') ||
            form.hasAttribute('hx-post') ||
            form.hasAttribute('hx-boost') ||
            form.closest('[hx-boost="true"]') !== null;
    }

    function resetPageParam(form) {
        let pageInput = form.querySelector('input[name="page"]');
        if (pageInput) {
            pageInput.value = '0';
        } else {
            let hidden = document.createElement('input');
            hidden.type = 'hidden';
            hidden.name = 'page';
            hidden.value = '0';
            form.appendChild(hidden);
        }
    }

    function submitForm(form) {
        if (typeof form.requestSubmit === 'function') {
            form.requestSubmit();
        } else {
            form.submit();
        }
    }

    function attachInput(input) {
        if (input.dataset._liveSearchBound) return;
        input.dataset._liveSearchBound = '1';
        let form = input.form;
        if (!form || form.method.toLowerCase() !== 'get') return;
        // HTMX-controlled forms have their own debounced trigger.
        if (isHtmxControlled(form)) return;

        let timer = null;
        input.addEventListener('input', function () {
            if (timer) clearTimeout(timer);
            timer = setTimeout(function () {
                resetPageParam(form);
                submitForm(form);
            }, DEBOUNCE_MS);
        });

        input.addEventListener('keydown', function (e) {
            if (e.key === 'Escape' && input.value !== '') {
                e.preventDefault();
                input.value = '';
                if (timer) clearTimeout(timer);
                submitForm(form);
            }
        });
    }

    function attachSelect(sel) {
        if (sel.dataset._liveSelectBound) return;
        sel.dataset._liveSelectBound = '1';
        let form = sel.form;
        if (!form || form.method.toLowerCase() !== 'get') return;
        if (isHtmxControlled(form)) return;
        let isFilterForm = !!form.querySelector('input[name="q"]') ||
            sel.name === 'per' ||
            sel.classList.contains('per-page-select');
        if (!isFilterForm) return;

        sel.addEventListener('change', function () {
            resetPageParam(form);
            submitForm(form);
        });
    }

    function init() {
        findQInputs().forEach(attachInput);
        document.querySelectorAll('form select').forEach(attachSelect);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();

// ── Client-Side Table Sorting ─────────────────────────────────
// Per backend convention: sorting that only re-orders rows already on the
// page is the FE's job. We sort the rows in #list-results (or the closest
// table to the clicked header) in-place — no round-trip, no HTMX swap.
//
// Each <th data-sortable data-sort-key="..."> participates. A cell can opt
// in to a custom comparable value via <td data-sort-value="..."> (used for
// preformatted dates, decimals with locale separators, etc.). Otherwise the
// trimmed textContent is used.
(function () {
    function nextDirection(current) {
        if (current === 'asc') return 'desc';
        if (current === 'desc') return '';
        return 'asc';
    }

    function indicatorFor(dir) {
        if (dir === 'asc') return '↑';
        if (dir === 'desc') return '↓';
        return '⇅';
    }

    function cellValue(row, idx) {
        let td = row.cells[idx];
        if (!td) return '';
        if (td.dataset && td.dataset.sortValue !== undefined) return td.dataset.sortValue;
        return (td.textContent || '').trim();
    }

    function asNumber(s) {
        if (s === '' || s == null) return NaN;
        // Strip thousands separators (comma, narrow no-break space, regular space)
        // and Arabic thousands separator. Keep one decimal point.
        let cleaned = String(s).replace(/[\s,\u00A0\u202F]/g, '');
        // Convert Arabic-Indic digits to ASCII
        cleaned = cleaned.replace(/[\u0660-\u0669]/g, function (d) {
            return String(d.charCodeAt(0) - 0x0660);
        }).replace(/[\u06F0-\u06F9]/g, function (d) {
            return String(d.charCodeAt(0) - 0x06F0);
        });
        if (!/^-?\d+(\.\d+)?$/.test(cleaned)) return NaN;
        return parseFloat(cleaned);
    }

    function compare(aRaw, bRaw, dir) {
        let aEmpty = (aRaw === '' || aRaw == null);
        let bEmpty = (bRaw === '' || bRaw == null);
        if (aEmpty && bEmpty) return 0;
        // Empties always sort first (matches BE "blanks before non-empty on asc")
        if (aEmpty) return dir === 'desc' ? 1 : -1;
        if (bEmpty) return dir === 'desc' ? -1 : 1;

        let an = asNumber(aRaw);
        let bn = asNumber(bRaw);
        let cmp;
        if (!isNaN(an) && !isNaN(bn)) {
            cmp = an - bn;
        } else {
            cmp = String(aRaw).localeCompare(String(bRaw), undefined, { sensitivity: 'base', numeric: true });
        }
        return dir === 'desc' ? -cmp : cmp;
    }

    function findTable(th) {
        let t = th;
        while (t && t.tagName !== 'TABLE') t = t.parentNode;
        return t;
    }

    function headerIndex(th) {
        let tr = th.parentNode;
        if (!tr) return -1;
        let i = 0;
        for (let c = 0; c < tr.cells.length; c++) {
            if (tr.cells[c] === th) return i;
            i += (tr.cells[c].colSpan || 1);
        }
        return -1;
    }

    function applySort(th) {
        let table = findTable(th);
        if (!table) return;
        let tbody = table.tBodies && table.tBodies[0];
        if (!tbody) return;

        let idx = headerIndex(th);
        if (idx < 0) return;

        let dir = (th.dataset.sortCurrent || '').toLowerCase();
        // Clear sibling indicators
        let siblings = table.querySelectorAll('th[data-sortable]');
        siblings.forEach(function (s) {
            if (s !== th) {
                s.dataset.sortCurrent = '';
                let ind = s.querySelector('.sort-indicator');
                if (ind) ind.textContent = indicatorFor('');
            }
        });

        // Always reflect the chosen direction on this header so the user
        // sees the click took effect, even when there's nothing to reorder
        // (empty list / placeholder row only).
        let indSelf = th.querySelector('.sort-indicator');
        if (indSelf) indSelf.textContent = indicatorFor(dir);

        // Skip placeholder rows (e.g. "no data" with single full-width cell).
        let rows = Array.prototype.slice.call(tbody.rows).filter(function (r) {
            return r.cells.length > 1;
        });
        if (rows.length === 0) return;

        if (dir === '') {
            // Restore original DOM order (cached on first sort).
            if (tbody._originalOrder) {
                tbody._originalOrder.forEach(function (r) { tbody.appendChild(r); });
            }
            return;
        }

        if (!tbody._originalOrder) {
            tbody._originalOrder = Array.prototype.slice.call(tbody.rows);
        }

        rows.sort(function (a, b) {
            return compare(cellValue(a, idx), cellValue(b, idx), dir);
        });
        rows.forEach(function (r) { tbody.appendChild(r); });
    }

    function initSortable() {
        let headers = document.querySelectorAll('th[data-sortable]');
        headers.forEach(function (th) {
            if (th.dataset._sortBound) return;
            th.dataset._sortBound = '1';
            th.style.cursor = 'pointer';
            th.style.userSelect = 'none';

            // Server may still emit data-sort-current; treat as initial state
            // but BE returns rows in canonical order so empty is the truth.
            th.dataset.sortCurrent = '';

            let indicator = th.querySelector('.sort-indicator');
            if (!indicator) {
                indicator = document.createElement('span');
                indicator.className = 'sort-indicator mr-1 text-gray-400 text-xs';
                th.prepend(indicator);
            }
            indicator.textContent = indicatorFor('');

            let key = th.dataset.sortKey || '';
            if (!key) return;

            th.addEventListener('click', function () {
                th.dataset.sortCurrent = nextDirection(th.dataset.sortCurrent || '');
                applySort(th);
            });
        });
    }

    document.addEventListener('DOMContentLoaded', initSortable);
    document.addEventListener('htmx:afterSwap', initSortable);
})();

// ── Table Scroll Shadow Indicators (C1) ───────────────────────
(function () {
    function updateShadows(wrapper) {
        let sl = wrapper.scrollLeft;
        let maxScroll = wrapper.scrollWidth - wrapper.clientWidth;
        // RTL: scrollLeft can be 0 (rightmost) or negative (leftmost)
        // In LTR: scrollLeft 0 = leftmost. But our layout is RTL.
        // For RTL containers, scrollLeft is 0 at the right edge (start)
        // and negative towards the left edge.
        let isRTL = getComputedStyle(wrapper).direction === 'rtl';
        if (isRTL) {
            // Normalize: some browsers use negative scrollLeft for RTL
            let absScroll = Math.abs(sl);
            wrapper.classList.toggle('scroll-right', absScroll > 2);
            wrapper.classList.toggle('scroll-left', absScroll < maxScroll - 2);
        } else {
            wrapper.classList.toggle('scroll-left', sl > 2);
            wrapper.classList.toggle('scroll-right', sl < maxScroll - 2);
        }
    }

    function initScrollShadows() {
        let wrappers = document.querySelectorAll('.data-table-wrapper');
        wrappers.forEach(function (wrapper) {
            if (wrapper.dataset._shadowBound) return;
            wrapper.dataset._shadowBound = '1';
            wrapper.addEventListener('scroll', function () { updateShadows(wrapper); });
            // Initial check
            updateShadows(wrapper);
        });
    }

    document.addEventListener('DOMContentLoaded', initScrollShadows);
    document.addEventListener('htmx:afterSwap', initScrollShadows);
    window.addEventListener('resize', function () {
        document.querySelectorAll('.data-table-wrapper').forEach(updateShadows);
    });
})();

// ── Action Overflow Dropdown (C2) ─────────────────────────────
(function () {
    function initActionDropdowns() {
        let btns = document.querySelectorAll('.action-overflow-btn');
        btns.forEach(function (btn) {
            if (btn.dataset._dropBound) return;
            btn.dataset._dropBound = '1';
            btn.addEventListener('click', function (e) {
                e.stopPropagation();
                let links = btn.parentElement.querySelector('.action-links');
                if (!links) return;
                // Close all other open dropdowns
                document.querySelectorAll('.action-links.show').forEach(function (d) {
                    if (d !== links) d.classList.remove('show');
                });
                links.classList.toggle('show');
            });
        });
    }

    // Close dropdowns on outside click
    document.addEventListener('click', function () {
        document.querySelectorAll('.action-links.show').forEach(function (d) {
            d.classList.remove('show');
        });
    });

    document.addEventListener('DOMContentLoaded', initActionDropdowns);
    document.addEventListener('htmx:afterSwap', initActionDropdowns);
})();
