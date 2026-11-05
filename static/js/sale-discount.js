// Per-line sale discounts in SAR. A value discount is an amount in SAR and a percent discount is a share of its line
// total. amount() keeps the result between 0 and the line total, and a value that is not a finite number counts as 0.
// The htmx listener sends products_discount and manual_discount in row order on forms marked data-sale-discounts. It
// reads disabled rows too, so each discount keeps its line slot.
(function (root, install) {
  if (typeof module === 'object' && module.exports) {
    module.exports = install;
  } else {
    install(root);
  }
})(typeof window !== 'undefined' ? window : null, function installSaleDiscount(window) {
  var document = window.document;
  function finiteOrZero(n) {
    return Number.isFinite(n) ? n : 0;
  }

  function amount(raw, mode, lineTotal) {
    var total = Math.max(finiteOrZero(lineTotal), 0);
    var value = finiteOrZero(parseFloat(raw));
    if (mode === 'pct') value = (value / 100) * total;
    return Number(Math.min(Math.max(value, 0), total).toFixed(2));
  }

  function fieldsNamed(form, name) {
    return Array.from(form.elements).filter(function (el) {
      return el.name === name;
    });
  }

  function cell(fields, i) {
    return fields[i] ? fields[i].value : '';
  }

  function lineDiscounts(form, prefix) {
    var prices = fieldsNamed(form, prefix + '_price');
    var quantities = fieldsNamed(form, prefix + '_quantity');
    return fieldsNamed(form, prefix + '_discount').map(function (discount, i) {
      var lineTotal = parseFloat(cell(prices, i)) * parseFloat(cell(quantities, i));
      return String(amount(discount.value, discount.dataset.mode, lineTotal));
    });
  }

  document.addEventListener('htmx:configRequest', function (evt) {
    var form = evt.detail.elt;
    if (form.dataset.saleDiscounts === undefined) return;
    evt.detail.parameters.products_discount = lineDiscounts(form, 'products');
    evt.detail.parameters.manual_discount = lineDiscounts(form, 'manual');
  });

  window.AfritaSaleDiscount = { amount: amount };
});
