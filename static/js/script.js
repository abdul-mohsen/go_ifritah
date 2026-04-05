// ── Toast Notification System ──────────────────────────────
(function() {
    var TOAST_DURATION = 5000;
    var ICONS = {
        error:   '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z"/></svg>',
        success: '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>',
        warning: '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01M10.29 3.86l-8.6 14.86A1 1 0 002.56 20h18.88a1 1 0 00.87-1.28l-8.6-14.86a1 1 0 00-1.72 0z"/></svg>',
        info:    '<svg class="w-5 h-5 shrink-0" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4m0-4h.01"/></svg>'
    };

    window.showToast = function(message, type) {
        type = type || 'error';
        if (!message) message = 'حدث خطأ، يرجى المحاولة مرة أخرى';
        var container = document.getElementById('toast-container');
        if (!container) return;

        var toast = document.createElement('div');
        toast.className = 'toast toast-' + type;
        toast.innerHTML = (ICONS[type] || ICONS.error) +
            '<span>' + message + '</span>' +
            '<button class="toast-close">&times;</button>';

        toast.querySelector('.toast-close').onclick = function() {
            toast.classList.add('toast-out');
            setTimeout(function() { toast.remove(); }, 300);
        };

        container.appendChild(toast);

        // Auto-remove
        var ref = toast;
        setTimeout(function() {
            if (ref.parentElement) {
                ref.classList.add('toast-out');
                setTimeout(function() { ref.remove(); }, 300);
            }
        }, TOAST_DURATION);
    };
})();

// ── PDF Opener (fetches PDF via JS, shows toast on error) ─────────
(function() {
    function handlePDF(url, action) {
        // Pre-open the window synchronously (during the click event) so popup
        // blockers don't suppress it. We navigate or close it after fetch.
        var win = null;
        if (action === 'open') {
            win = window.open('about:blank', '_blank');
        }

        fetch(url, { credentials: 'same-origin' })
            .then(function(resp) {
                var ct = resp.headers.get('content-type') || '';
                if (resp.ok && ct.indexOf('application/pdf') !== -1) {
                    return resp.blob().then(function(blob) {
                        var blobUrl = URL.createObjectURL(blob);
                        if (action === 'print') {
                            var iframe = document.createElement('iframe');
                            iframe.style.position = 'fixed';
                            iframe.style.right = '-9999px';
                            iframe.style.width = '0';
                            iframe.style.height = '0';
                            iframe.src = blobUrl;
                            document.body.appendChild(iframe);
                            iframe.onload = function() {
                                try {
                                    iframe.contentWindow.focus();
                                    iframe.contentWindow.print();
                                } catch(e) {}
                                // Use afterprint event to clean up after user finishes
                                // Falls back to a long timeout if afterprint isn't supported
                                var cleaned = false;
                                function cleanup() {
                                    if (cleaned) return;
                                    cleaned = true;
                                    try { document.body.removeChild(iframe); } catch(e) {}
                                    URL.revokeObjectURL(blobUrl);
                                }
                                try {
                                    iframe.contentWindow.addEventListener('afterprint', cleanup);
                                } catch(e) {}
                                // Fallback: clean up after 5 minutes if afterprint never fires
                                setTimeout(cleanup, 300000);
                            };
                        } else if (win) {
                            win.location.href = blobUrl;
                            // Revoke after a delay so the new tab can render
                            setTimeout(function() { URL.revokeObjectURL(blobUrl); }, 120000);
                        }
                    });
                }
                // Error response — close pre-opened window and show toast
                if (win) { try { win.close(); } catch(e) {} }
                return resp.text().then(function(text) {
                    var msg = 'تعذر تحميل ملف PDF، يرجى المحاولة لاحقاً';
                    try {
                        var json = JSON.parse(text);
                        if (json.message) msg = json.message;
                    } catch(e) {}
                    window.showToast(msg, 'error');
                });
            })
            .catch(function() {
                if (win) { try { win.close(); } catch(e) {} }
                window.showToast('تعذر الاتصال بالخادم، يرجى المحاولة لاحقاً', 'error');
            });
    }

    window.openPDF = function(url) { handlePDF(url, 'open'); };
    window.printPDF = function(url) { handlePDF(url, 'print'); };
})();

// ── Flash Cookie Reader (shows toast from redirected pages) ───────
(function() {
    var match = document.cookie.match(/(?:^|;\s*)afrita_flash=([^;]*)/);
    if (match) {
        try {
            var flash = JSON.parse(decodeURIComponent(match[1]));
            if (flash.message && window.showToast) {
                // Small delay to ensure DOM is ready
                setTimeout(function() {
                    window.showToast(flash.message, flash.type || 'success');
                }, 100);
            }
        } catch(e) {}
        // Clear the cookie
        document.cookie = 'afrita_flash=; path=/; max-age=0';
    }
})();

// ── Global Loading Overlay ────────────────────────────────────
(function() {
    var loadingEl = document.getElementById('global-loading');
    var showTimer = null;

    window.__showLoading = function() {
        // Debounce: only show after 200ms to avoid flicker on fast requests
        if (showTimer) return;
        showTimer = setTimeout(function() {
            if (loadingEl) {
                loadingEl.style.display = '';
                loadingEl.classList.remove('hidden');
            }
        }, 200);
    };

    window.__hideLoading = function() {
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
(function() {
    var disabledButtons = [];

    document.addEventListener("htmx:beforeRequest", function(evt) {
        // Show global loading
        if (window.__showLoading) window.__showLoading();

        // Disable all submit buttons in the requesting form
        var form = evt.detail.elt;
        if (form && form.tagName !== 'FORM') form = form.closest('form');
        if (form) {
            var btns = form.querySelectorAll('button[type="submit"], button:not([type])');
            btns.forEach(function(btn) {
                btn.disabled = true;
                btn.classList.add('htmx-requesting');
            });
            disabledButtons = disabledButtons.concat(Array.from(btns));
        }
    });

    function reEnableButtons() {
        if (window.__hideLoading) window.__hideLoading();
        disabledButtons.forEach(function(btn) {
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
document.addEventListener("htmx:confirm", function(evt) {
    // Only confirm deletions
    var verb = evt.detail.requestConfig ? evt.detail.requestConfig.verb : '';
    var path = evt.detail.path || '';
    if (verb === 'delete' || path.indexOf('/delete') !== -1) {
        if (!confirm("هل أنت متأكد من الحذف؟")) {
            evt.preventDefault();
        }
    }
});

// Intercept non-2xx HTMX responses — show toast instead of swapping raw error text
document.addEventListener("htmx:beforeSwap", function(evt) {
    var xhr = evt.detail.xhr;
    if (xhr && xhr.status >= 400) {
        evt.detail.shouldSwap = false;
        if (window.__hideLoading) window.__hideLoading();

        // Extract message from response
        var msg = '';
        var text = (xhr.responseText || '').trim();
        if (text) {
            // Try JSON: {"detail":"..."} or {"error":"..."} or {"message":"..."}
            try {
                var json = JSON.parse(text);
                msg = json.detail || json.error || json.message || json.msg || '';
            } catch(e) {
                // Use plain text if short and not HTML
                if (text.length < 200 && text.charAt(0) !== '<') {
                    msg = text;
                }
            }
        }

        // Provide meaningful fallback based on status code
        if (!msg) {
            var statusMessages = {
                400: 'البيانات المرسلة غير صحيحة',
                401: 'انتهت صلاحية الجلسة، يرجى تسجيل الدخول مجدداً',
                403: 'ليس لديك صلاحية لتنفيذ هذا الإجراء',
                404: 'العنصر المطلوب غير موجود',
                409: 'يوجد تعارض في البيانات، يرجى المحاولة مجدداً',
                422: 'البيانات المرسلة غير مكتملة',
                429: 'طلبات كثيرة جداً، يرجى الانتظار قليلاً',
                500: 'حدث خطأ في الخادم، يرجى المحاولة لاحقاً',
                502: 'الخادم غير متاح حالياً',
                503: 'الخدمة غير متاحة مؤقتاً، يرجى المحاولة لاحقاً'
            };
            msg = statusMessages[xhr.status] || 'حدث خطأ، يرجى المحاولة مرة أخرى';
        }

        window.showToast(msg, 'error');

        // Auto-redirect to login on 401
        if (xhr.status === 401) {
            setTimeout(function() { window.location.href = '/login'; }, 2000);
        }
    }
});

// Handle network errors (backend unreachable)
document.addEventListener("htmx:responseError", function() {
    if (window.__hideLoading) window.__hideLoading();
    window.showToast('تعذر الاتصال بالخادم، يرجى التحقق من الاتصال والمحاولة لاحقاً', 'error');
});

// Handle request send failures (timeout, connection refused)
document.addEventListener("htmx:sendError", function() {
    if (window.__hideLoading) window.__hideLoading();
    window.showToast('فشل إرسال الطلب، يرجى التحقق من اتصالك بالإنترنت', 'error');
});

// Listen for HX-Trigger showToast events (sent by backend)
document.addEventListener("showToast", function(evt) {
    var d = evt.detail || {};
    window.showToast(d.message || '', d.type || 'error');
});

// Hide loading indicator + auto-hide alerts after swap
document.addEventListener("htmx:afterSwap", function(evt) {
    if (window.__hideLoading) window.__hideLoading();
    // Auto-hide alerts after 3 seconds
    var alerts = document.querySelectorAll("[role='alert']");
    alerts.forEach(function(alert) {
        setTimeout(function() {
            alert.style.transition = 'opacity 0.3s ease';
            alert.style.opacity = '0';
            setTimeout(function() { alert.remove(); }, 300);
        }, 3000);
    });
});

// ── Keyboard Shortcuts ────────────────────────────────────────
(function() {
    document.addEventListener('keydown', function(e) {
        // Ctrl+K or Cmd+K → focus search input
        if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
            e.preventDefault();
            var searchInput = document.querySelector('input[name="q"]');
            if (searchInput) {
                searchInput.focus();
                searchInput.select();
            }
        }
        // Escape → close open modals
        if (e.key === 'Escape') {
            var modals = document.querySelectorAll('.fixed:not(.hidden)');
            modals.forEach(function(modal) {
                if (modal.id && modal.id !== 'global-loading' && modal.id !== 'sidebar-overlay') {
                    modal.classList.add('hidden');
                }
            });
        }
    });
})();

// ── Delete Success Toast ──────────────────────────────────────
(function() {
    document.addEventListener('htmx:afterRequest', function(evt) {
        var xhr = evt.detail.xhr;
        var path = evt.detail.pathInfo ? evt.detail.pathInfo.requestPath : '';
        var verb = (evt.detail.requestConfig && evt.detail.requestConfig.verb) || '';
        if (xhr && xhr.status >= 200 && xhr.status < 300 &&
            (verb === 'delete' || path.indexOf('/delete') !== -1)) {
            window.showToast('تم الحذف بنجاح', 'success');
        }
    });
})();

// ── Search Form Loading State ──────────────────────────────────
(function() {
    document.addEventListener('submit', function(e) {
        var form = e.target;
        if (form.tagName !== 'FORM' || form.method !== 'get') return;
        var btn = form.querySelector('button[type="submit"], button:not([type])');
        if (!btn) return;
        btn.disabled = true;
        btn.innerHTML = '<svg class="animate-spin h-4 w-4 inline-block" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/></svg>';
        if (window.__showLoading) window.__showLoading();
    });
})();

// ── Client-Side Table Sorting ─────────────────────────────────
(function() {
    function initSortable() {
        var headers = document.querySelectorAll('th[data-sortable]');
        headers.forEach(function(th) {
            if (th.dataset._sortBound) return;
            th.dataset._sortBound = '1';
            th.style.cursor = 'pointer';
            th.style.userSelect = 'none';
            // Add sort indicator
            var indicator = document.createElement('span');
            indicator.className = 'sort-indicator mr-1 text-gray-400 text-xs';
            indicator.textContent = '⇅';
            th.prepend(indicator);

            th.addEventListener('click', function() {
                var table = th.closest('table');
                if (!table) return;
                var tbody = table.querySelector('tbody');
                if (!tbody) return;
                var colIndex = Array.from(th.parentNode.children).indexOf(th);
                var rows = Array.from(tbody.querySelectorAll('tr'));

                // Determine direction
                var asc = th.dataset.sortDir !== 'asc';
                // Reset other headers
                var allTh = table.querySelectorAll('th[data-sortable]');
                allTh.forEach(function(h) {
                    h.dataset.sortDir = '';
                    var ind = h.querySelector('.sort-indicator');
                    if (ind) ind.textContent = '⇅';
                });
                th.dataset.sortDir = asc ? 'asc' : 'desc';
                indicator.textContent = asc ? '↑' : '↓';

                rows.sort(function(a, b) {
                    var aCell = a.children[colIndex];
                    var bCell = b.children[colIndex];
                    if (!aCell || !bCell) return 0;
                    var aText = (aCell.textContent || '').trim();
                    var bText = (bCell.textContent || '').trim();
                    // Try numeric comparison
                    var aNum = parseFloat(aText.replace(/[^\d.\-]/g, ''));
                    var bNum = parseFloat(bText.replace(/[^\d.\-]/g, ''));
                    if (!isNaN(aNum) && !isNaN(bNum)) {
                        return asc ? aNum - bNum : bNum - aNum;
                    }
                    // String comparison
                    return asc ? aText.localeCompare(bText, 'ar') : bText.localeCompare(aText, 'ar');
                });
                rows.forEach(function(row) { tbody.appendChild(row); });
            });
        });
    }

    document.addEventListener('DOMContentLoaded', initSortable);
    document.addEventListener('htmx:afterSwap', initSortable);
})();

// ── Table Scroll Shadow Indicators (C1) ───────────────────────
(function() {
    function updateShadows(wrapper) {
        var sl = wrapper.scrollLeft;
        var maxScroll = wrapper.scrollWidth - wrapper.clientWidth;
        // RTL: scrollLeft can be 0 (rightmost) or negative (leftmost)
        // In LTR: scrollLeft 0 = leftmost. But our layout is RTL.
        // For RTL containers, scrollLeft is 0 at the right edge (start)
        // and negative towards the left edge.
        var isRTL = getComputedStyle(wrapper).direction === 'rtl';
        if (isRTL) {
            // Normalize: some browsers use negative scrollLeft for RTL
            var absScroll = Math.abs(sl);
            wrapper.classList.toggle('scroll-right', absScroll > 2);
            wrapper.classList.toggle('scroll-left', absScroll < maxScroll - 2);
        } else {
            wrapper.classList.toggle('scroll-left', sl > 2);
            wrapper.classList.toggle('scroll-right', sl < maxScroll - 2);
        }
    }

    function initScrollShadows() {
        var wrappers = document.querySelectorAll('.data-table-wrapper');
        wrappers.forEach(function(wrapper) {
            if (wrapper.dataset._shadowBound) return;
            wrapper.dataset._shadowBound = '1';
            wrapper.addEventListener('scroll', function() { updateShadows(wrapper); });
            // Initial check
            updateShadows(wrapper);
        });
    }

    document.addEventListener('DOMContentLoaded', initScrollShadows);
    document.addEventListener('htmx:afterSwap', initScrollShadows);
    window.addEventListener('resize', function() {
        document.querySelectorAll('.data-table-wrapper').forEach(updateShadows);
    });
})();

// ── Action Overflow Dropdown (C2) ─────────────────────────────
(function() {
    function initActionDropdowns() {
        var btns = document.querySelectorAll('.action-overflow-btn');
        btns.forEach(function(btn) {
            if (btn.dataset._dropBound) return;
            btn.dataset._dropBound = '1';
            btn.addEventListener('click', function(e) {
                e.stopPropagation();
                var links = btn.parentElement.querySelector('.action-links');
                if (!links) return;
                // Close all other open dropdowns
                document.querySelectorAll('.action-links.show').forEach(function(d) {
                    if (d !== links) d.classList.remove('show');
                });
                links.classList.toggle('show');
            });
        });
    }

    // Close dropdowns on outside click
    document.addEventListener('click', function() {
        document.querySelectorAll('.action-links.show').forEach(function(d) {
            d.classList.remove('show');
        });
    });

    document.addEventListener('DOMContentLoaded', initActionDropdowns);
    document.addEventListener('htmx:afterSwap', initActionDropdowns);
})();
