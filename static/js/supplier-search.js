// Supplier search dropdown functionality
(function() {
    var input = document.getElementById('supplier_search_input');
    var results = document.getElementById('supplier_search_results');
    var select = document.querySelector('[name="supplier_id"]');
    if (!input || !select || !results) {
        return;
    }

    function getSuppliers() {
        var list = [];
        var opts = select.querySelectorAll('option');
        for (var i = 0; i < opts.length; i++) {
            if (opts[i].value) {
                var name = opts[i].getAttribute('data-name') || opts[i].textContent;
                list.push({id: opts[i].value, name: name});
            }
        }
        return list;
    }

    function filter(q) {
        if (!q) {
            return [];
        }
        var lower = q.toLowerCase();
        return getSuppliers().filter(function(s) {
            return s.name.toLowerCase().includes(lower);
        });
    }

    function render(items) {
        if (!items.length) {
            results.innerHTML = '';
            results.classList.add('hidden');
            return;
        }
        var html = '';
        for (var i = 0; i < items.length; i++) {
            html += '<div class="supplier-search-item px-3 py-2 text-sm hover:bg-blue-50 cursor-pointer" data-id="' + escapeAttr(items[i].id) + '">' + escapeHtml(items[i].name) + '</div>';
        }
        results.innerHTML = html;
        results.classList.remove('hidden');
    }

    input.addEventListener('input', function() {
        render(filter(this.value));
    });
    input.addEventListener('blur', function() {
        setTimeout(function() {
            results.classList.add('hidden');
        }, 100);
    });

    results.addEventListener('click', function(e) {
        var item = e.target.closest('.supplier-search-item');
        if (!item) {
            return;
        }
        input.value = item.textContent;
        select.value = item.getAttribute('data-id');
        results.classList.add('hidden');
        select.dispatchEvent(new Event('change', {bubbles: true}));
    });

    var first = select.querySelector('option[value]');
    if (first) {
        select.value = first.value;
        var name = first.getAttribute('data-name') || first.textContent;
        input.value = name;
    }
})();
