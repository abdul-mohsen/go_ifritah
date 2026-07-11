// Purchase-bill supplier combobox.
(function () {
    var root = document.querySelector('[data-supplier-combobox]');
    if (!root) return;

    var input = root.querySelector('[data-supplier-search-input]');
    var results = root.querySelector('[data-supplier-results]');
    var select = root.querySelector('[data-supplier-select]');
    if (!input || !results || !select) return;

    var activeIndex = -1;
    var pointerSelecting = false;
    var emptyLabel = root.getAttribute('data-empty-label') || 'No matching suppliers';

    function normalize(value) {
        if (value == null) return '';
        value = String(value).toLowerCase().trim();
        if (!value) return '';
        value = value.replace(/[\u064B-\u0652\u0640]/g, '');
        value = value.replace(/[\u0623\u0625\u0622]/g, '\u0627');
        value = value.replace(/\u0629/g, '\u0647');
        value = value.replace(/\u0649/g, '\u064A');
        value = value.replace(/[\u0660-\u0669]/g, function (digit) {
            return String.fromCharCode(digit.charCodeAt(0) - 0x0660 + 0x30);
        });
        value = value.replace(/[\u06F0-\u06F9]/g, function (digit) {
            return String.fromCharCode(digit.charCodeAt(0) - 0x06F0 + 0x30);
        });
        return value.replace(/\s+/g, ' ');
    }

    function escapeHtml(value) {
        var div = document.createElement('div');
        div.textContent = value == null ? '' : String(value);
        return div.innerHTML;
    }

    function suppliers() {
        var list = [];
        var options = select.querySelectorAll('option');
        for (var i = 0; i < options.length; i++) {
            var option = options[i];
            if (!option.value) continue;
            list.push({
                id: option.value,
                name: (option.getAttribute('data-name') || option.textContent || '').trim()
            });
        }
        return list;
    }

    function selectedSupplier() {
        var current = select.querySelector('option:checked');
        if (!current || !current.value) return null;
        return {
            id: current.value,
            name: (current.getAttribute('data-name') || current.textContent || '').trim()
        };
    }

    function filteredSuppliers(query) {
        var normalizedQuery = normalize(query);
        var list = suppliers();
        if (!normalizedQuery) return list;

        var terms = normalizedQuery.split(' ');
        return list.filter(function (supplier) {
            var haystack = normalize(supplier.name);
            for (var i = 0; i < terms.length; i++) {
                if (terms[i] && haystack.indexOf(terms[i]) === -1) {
                    return false;
                }
            }
            return true;
        });
    }

    function closeResults() {
        results.classList.add('hidden');
        input.setAttribute('aria-expanded', 'false');
        activeIndex = -1;
    }

    function syncInputToSelection() {
        var selected = selectedSupplier();
        input.value = selected ? selected.name : '';
    }

    function render(query) {
        var list = filteredSuppliers(query);
        var selected = selectedSupplier();

        if (!list.length) {
            results.innerHTML = '<div class="px-3 py-2 text-sm text-gray-500">' + escapeHtml(emptyLabel) + '</div>';
            results.classList.remove('hidden');
            input.setAttribute('aria-expanded', 'true');
            activeIndex = -1;
            return;
        }

        var html = '';
        for (var i = 0; i < list.length; i++) {
            var isSelected = selected && selected.id === list[i].id;
            html += '<div class="supplier-search-item px-3 py-2 text-sm cursor-pointer' +
                (isSelected ? ' bg-blue-50 text-blue-700' : ' hover:bg-blue-50') +
                '" role="option" aria-selected="' + (isSelected ? 'true' : 'false') + '"' +
                ' data-id="' + escapeHtml(list[i].id) + '"' +
                ' data-name="' + escapeHtml(list[i].name) + '">' +
                escapeHtml(list[i].name) +
                '</div>';
        }

        results.innerHTML = html;
        results.classList.remove('hidden');
        input.setAttribute('aria-expanded', 'true');
        activeIndex = -1;
    }

    function supplierItems() {
        return results.querySelectorAll('.supplier-search-item');
    }

    function setActiveIndex(index) {
        var items = supplierItems();
        if (!items.length) {
            activeIndex = -1;
            return;
        }

        if (index < 0) index = items.length - 1;
        if (index >= items.length) index = 0;
        activeIndex = index;

        for (var i = 0; i < items.length; i++) {
            items[i].classList.toggle('bg-blue-100', i === activeIndex);
        }
        items[activeIndex].scrollIntoView({ block: 'nearest' });
    }

    function commitSelection(id, name) {
        if (!id || !name) return;
        var changed = select.value !== id;
        select.value = id;
        input.value = name;
        closeResults();
        if (changed) {
            select.dispatchEvent(new Event('change', { bubbles: true }));
        }
    }

    function firstSupplier() {
        var list = suppliers();
        return list.length ? list[0] : null;
    }

    if (!selectedSupplier()) {
        var first = firstSupplier();
        if (first) {
            select.value = first.id;
        }
    }
    syncInputToSelection();

    input.addEventListener('focus', function () {
        render('');
    });

    input.addEventListener('input', function () {
        render(input.value);
    });

    input.addEventListener('keydown', function (event) {
        var items = supplierItems();
        if (event.key === 'ArrowDown') {
            event.preventDefault();
            if (results.classList.contains('hidden')) {
                render(input.value);
                items = supplierItems();
            }
            setActiveIndex(activeIndex + 1);
            return;
        }
        if (event.key === 'ArrowUp') {
            event.preventDefault();
            if (results.classList.contains('hidden')) {
                render(input.value);
                items = supplierItems();
            }
            setActiveIndex(activeIndex - 1);
            return;
        }
        if (event.key === 'Enter' && !results.classList.contains('hidden')) {
            items = supplierItems();
            if (items.length) {
                event.preventDefault();
                if (activeIndex < 0) {
                    setActiveIndex(0);
                }
                var active = supplierItems()[activeIndex];
                if (active) {
                    commitSelection(active.getAttribute('data-id'), active.getAttribute('data-name'));
                }
            }
            return;
        }
        if (event.key === 'Escape') {
            closeResults();
            syncInputToSelection();
        }
    });

    input.addEventListener('blur', function () {
        window.setTimeout(function () {
            if (pointerSelecting) return;
            closeResults();
            syncInputToSelection();
        }, 120);
    });

    results.addEventListener('mousedown', function (event) {
        if (event.target.closest('.supplier-search-item')) {
            pointerSelecting = true;
            event.preventDefault();
        }
    });

    results.addEventListener('mouseup', function () {
        pointerSelecting = false;
    });

    results.addEventListener('click', function (event) {
        var item = event.target.closest('.supplier-search-item');
        if (!item) return;
        commitSelection(item.getAttribute('data-id'), item.getAttribute('data-name'));
        pointerSelecting = false;
    });

    select.addEventListener('change', syncInputToSelection);

    document.addEventListener('click', function (event) {
        if (!root.contains(event.target)) {
            closeResults();
            syncInputToSelection();
        }
    });
})();
