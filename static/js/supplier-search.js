// Purchase-bill supplier combobox.
(() => {
    const root = document.querySelector('[data-supplier-combobox]');
    if (!root) return;

    const input = root.querySelector('[data-supplier-search-input]');
    const results = root.querySelector('[data-supplier-results]');
    const select = root.querySelector('[data-supplier-select]');
    if (!input || !results || !select) return;

    let activeIndex = -1;
    let pointerSelecting = false;
    const emptyLabel = root.dataset.emptyLabel || 'No matching suppliers';
    const preferredMaxHeight = 256;

    const normalizeArabicDigit = (digit, baseCodePoint) =>
        String.fromCodePoint(digit.codePointAt(0) - baseCodePoint + 0x30);

    const normalize = (value) => {
        if (value == null) return '';

        const normalized = String(value)
            .toLowerCase()
            .trim()
            .replaceAll(/[\u064B-\u0652\u0640]/g, '')
            .replaceAll(/[\u0623\u0625\u0622]/g, '\u0627')
            .replaceAll('\u0629', '\u0647')
            .replaceAll('\u0649', '\u064A')
            .replaceAll(/[\u0660-\u0669]/g, (digit) => normalizeArabicDigit(digit, 0x0660))
            .replaceAll(/[\u06F0-\u06F9]/g, (digit) => normalizeArabicDigit(digit, 0x06F0))
            .replaceAll(/\s+/g, ' ');

        return normalized;
    };

    const escapeHtml = (value) => {
        const element = document.createElement('div');
        element.textContent = value == null ? '' : String(value);
        return element.innerHTML;
    };

    const getSuppliers = () =>
        Array.from(select.querySelectorAll('option'))
            .filter((option) => option.value)
            .map((option) => ({
                id: option.value,
                name: (option.dataset.name || option.textContent || '').trim(),
            }));

    const getSelectedSupplier = () => {
        const current = select.querySelector('option:checked');
        if (!current || !current.value) return null;

        return {
            id: current.value,
            name: (current.dataset.name || current.textContent || '').trim(),
        };
    };

    const filterSuppliers = (query) => {
        const normalizedQuery = normalize(query);
        const suppliers = getSuppliers();
        if (!normalizedQuery) return suppliers;

        const terms = normalizedQuery.split(' ');
        return suppliers.filter((supplier) => {
            const haystack = normalize(supplier.name);
            return terms.every((term) => !term || haystack.includes(term));
        });
    };

    const closeResults = () => {
        results.classList.add('hidden');
        input.setAttribute('aria-expanded', 'false');
        activeIndex = -1;
    };

    const syncResultsWidth = () => {
        const width = input.offsetWidth;
        if (!width) return;

        const cssWidth = `${width}px`;
        results.style.left = '0';
        results.style.right = 'auto';
        results.style.width = cssWidth;
        results.style.minWidth = cssWidth;
        results.style.maxWidth = cssWidth;
        results.style.boxSizing = 'border-box';
    };

    const syncResultsMaxHeight = () => {
        const boundary = root.closest('form') || root.closest('.page-card-wide') || root.parentElement;
        if (!boundary) {
            results.style.maxHeight = `${preferredMaxHeight}px`;
            return;
        }

        const inputRect = input.getBoundingClientRect();
        const boundaryRect = boundary.getBoundingClientRect();
        const viewportBottom = window.innerHeight - 8;
        const boundaryBottom = Math.min(boundaryRect.bottom, viewportBottom);
        const availableHeight = Math.floor(boundaryBottom - inputRect.bottom - 8);

        if (availableHeight <= 0) {
            results.style.maxHeight = '0px';
            return;
        }

        results.style.maxHeight = `${Math.min(preferredMaxHeight, availableHeight)}px`;
    };

    const syncInputToSelection = () => {
        const selected = getSelectedSupplier();
        input.value = selected ? selected.name : '';
    };

    const renderEmptyState = () => {
        syncResultsWidth();
        syncResultsMaxHeight();
        results.innerHTML = `<div class="px-3 py-2 text-sm text-gray-500">${escapeHtml(emptyLabel)}</div>`;
        results.classList.remove('hidden');
        input.setAttribute('aria-expanded', 'true');
        activeIndex = -1;
    };

    const renderResults = (query) => {
        const filtered = filterSuppliers(query);
        const selected = getSelectedSupplier();

        if (!filtered.length) {
            renderEmptyState();
            return;
        }

        const html = filtered
            .map((supplier) => {
                const isSelected = selected?.id === supplier.id;
                const stateClass = isSelected ? ' bg-blue-50 text-blue-700' : ' hover:bg-blue-50';

                return `<div class="supplier-search-item px-3 py-2 text-sm cursor-pointer truncate${stateClass}" role="option" aria-selected="${isSelected ? 'true' : 'false'}" data-id="${escapeHtml(supplier.id)}" data-name="${escapeHtml(supplier.name)}" title="${escapeHtml(supplier.name)}">${escapeHtml(supplier.name)}</div>`;
            })
            .join('');

        syncResultsWidth();
        syncResultsMaxHeight();
        results.innerHTML = html;
        results.classList.remove('hidden');
        input.setAttribute('aria-expanded', 'true');
        activeIndex = -1;
    };

    const getSupplierItems = () => Array.from(results.querySelectorAll('.supplier-search-item'));

    const setActiveIndex = (index) => {
        const items = getSupplierItems();
        if (!items.length) {
            activeIndex = -1;
            return;
        }

        if (index < 0) index = items.length - 1;
        if (index >= items.length) index = 0;
        activeIndex = index;

        for (const [itemIndex, item] of items.entries()) {
            item.classList.toggle('bg-blue-100', itemIndex === activeIndex);
        }

        items[activeIndex].scrollIntoView({ block: 'nearest' });
    };

    const commitSelection = (id, name) => {
        if (!id || !name) return;

        const changed = select.value !== id;
        select.value = id;
        input.value = name;
        closeResults();

        if (changed) {
            select.dispatchEvent(new Event('change', { bubbles: true }));
        }
    };

    const firstSupplier = () => getSuppliers()[0] || null;

    if (!getSelectedSupplier()) {
        const first = firstSupplier();
        if (first) {
            select.value = first.id;
        }
    }

    syncResultsWidth();
    syncResultsMaxHeight();
    syncInputToSelection();

    input.addEventListener('focus', () => {
        renderResults('');
    });

    input.addEventListener('input', () => {
        renderResults(input.value);
    });

    input.addEventListener('keydown', (event) => {
        if (event.key === 'ArrowDown') {
            event.preventDefault();
            if (results.classList.contains('hidden')) {
                renderResults(input.value);
            }
            setActiveIndex(activeIndex + 1);
            return;
        }

        if (event.key === 'ArrowUp') {
            event.preventDefault();
            if (results.classList.contains('hidden')) {
                renderResults(input.value);
            }
            setActiveIndex(activeIndex - 1);
            return;
        }

        if (event.key === 'Enter' && !results.classList.contains('hidden')) {
            const items = getSupplierItems();
            if (!items.length) return;

            event.preventDefault();
            if (activeIndex < 0) {
                setActiveIndex(0);
            }

            const activeItem = getSupplierItems()[activeIndex];
            commitSelection(activeItem?.dataset.id, activeItem?.dataset.name);
            return;
        }

        if (event.key === 'Escape') {
            closeResults();
            syncInputToSelection();
        }
    });

    input.addEventListener('blur', () => {
        window.setTimeout(() => {
            if (pointerSelecting) return;
            closeResults();
            syncInputToSelection();
        }, 120);
    });

    results.addEventListener('mousedown', (event) => {
        if (event.target.closest('.supplier-search-item')) {
            pointerSelecting = true;
            event.preventDefault();
        }
    });

    results.addEventListener('mouseup', () => {
        pointerSelecting = false;
    });

    results.addEventListener('click', (event) => {
        const item = event.target.closest('.supplier-search-item');
        if (!item) return;

        commitSelection(item.dataset.id, item.dataset.name);
        pointerSelecting = false;
    });

    select.addEventListener('change', syncInputToSelection);
    window.addEventListener('resize', () => {
        syncResultsWidth();
        syncResultsMaxHeight();
    });

    document.addEventListener('click', (event) => {
        if (!root.contains(event.target)) {
            closeResults();
            syncInputToSelection();
        }
    });
})();
