/* supplier-search.js — searchable supplier dropdown for purchase bills form */

(function () {
  'use strict';

  function initSupplierSearch() {
    var searchInput = document.getElementById('supplier_search_input');
    var resultsContainer = document.getElementById('supplier_search_results');
    var hiddenSelect = document.getElementById('supplier_id_select');

    if (!searchInput || !resultsContainer || !hiddenSelect) return;

    var suppliers = [];
    // Build suppliers array from hidden select options
    hiddenSelect.querySelectorAll('option:not(:first-child)').forEach(function (opt) {
      suppliers.push({
        id: opt.value,
        name: opt.getAttribute('data-name') || opt.textContent.trim(),
        creditLimit: opt.getAttribute('data-credit-limit') || '0'
      });
    });

    function normalizeText(s) {
      return String(s || '').toLowerCase().trim()
        .replace(/[\u064B-\u0652\u0640]/g, '')  // diacritics
        .replace(/[\u0623\u0625\u0622]/g, '\u0627')  // أإآ → ا
        .replace(/\u0629/g, '\u0647')  // ة → ه
        .replace(/[\u0660-\u0669]/g, function (d) { return String.fromCharCode(d.charCodeAt(0) - 0x0660 + 0x30); })  // arabic-indic digits
        .replace(/[\u06F0-\u06F9]/g, function (d) { return String.fromCharCode(d.charCodeAt(0) - 0x06F0 + 0x30); });  // ext arabic-indic
    }

    function matchesQuery(supplier, query) {
      if (!query) return false;
      var q = normalizeText(query);
      var n = normalizeText(supplier.name);
      return n.indexOf(q) !== -1;  // substring match
    }

    function renderResults() {
      var query = searchInput.value.trim();
      var matches = query ? suppliers.filter(function (s) { return matchesQuery(s, query); }) : [];

      resultsContainer.innerHTML = '';
      if (!query) {
        resultsContainer.classList.add('hidden');
        return;
      }

      if (!matches.length) {
        var noMatch = document.createElement('div');
        noMatch.className = 'supplier-search-no-match';
        noMatch.textContent = 'لا توجد نتائج';
        resultsContainer.appendChild(noMatch);
        resultsContainer.classList.remove('hidden');
        return;
      }

      matches.forEach(function (supplier) {
        var item = document.createElement('button');
        item.type = 'button';
        item.className = 'supplier-search-item';
        item.textContent = supplier.name;
        item.setAttribute('data-id', supplier.id);
        item.addEventListener('click', function (e) {
          e.preventDefault();
          selectSupplier(supplier.id, supplier.name);
        });
        resultsContainer.appendChild(item);
      });

      resultsContainer.classList.remove('hidden');
    }

    function selectSupplier(id, name) {
      searchInput.value = name;
      hiddenSelect.value = id;
      hiddenSelect.dispatchEvent(new Event('change', { bubbles: true }));
      resultsContainer.classList.add('hidden');
      resultsContainer.innerHTML = '';
    }

    searchInput.addEventListener('input', renderResults);
    searchInput.addEventListener('focus', renderResults);

    document.addEventListener('click', function (e) {
      if (!searchInput.contains(e.target) && !resultsContainer.contains(e.target)) {
        resultsContainer.classList.add('hidden');
      }
    });

    // Pre-select first supplier if available
    if (suppliers.length > 0) {
      selectSupplier(suppliers[0].id, suppliers[0].name);
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initSupplierSearch);
  } else {
    initSupplierSearch();
  }
})();
