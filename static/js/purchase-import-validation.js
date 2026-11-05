(function (root, api) {
  if (typeof module === 'object' && module.exports) {
    module.exports = api;
  } else {
    root.AfritaPurchaseImportValidation = api;
  }
})(typeof window !== 'undefined' ? window : null, {
  parsePositiveInteger: function (value) {
    var text = String(value == null ? '' : value).trim();
    if (!/^\d+$/.test(text)) return 0;
    var parsed = Number(text);
    return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 0;
  },

  isNonNegativeNumber: function (value) {
    var text = String(value == null ? '' : value).trim();
    if (!/^(?:\d+(?:\.\d+)?|\.\d+)$/.test(text)) return false;
    return Number.isFinite(Number(text)) && Number(text) >= 0;
  },

  isPositiveNumber: function (value) {
    return this.isNonNegativeNumber(value) && Number(value) > 0;
  },

  hasProductInput: function (values) {
    return values.some(function (value) {
      return String(value == null ? '' : value).trim() !== '';
    });
  },

  validatePurchaseImportBills: function (bills) {
    for (var i = 0; i < bills.length; i++) {
      var payload = bills[i].payload;
      if (
        !this.parsePositiveInteger(payload.store_id) ||
        !this.parsePositiveInteger(payload.supplier_id) ||
        !this.parsePositiveInteger(payload.supplier_sequence_number) ||
        !this.parsePositiveInteger(payload.payment_method) ||
        !/^\d{4}-\d{2}-\d{2}T00:00:00\+03:00$/.test(payload.effective_date)
      ) {
        return 'purchase_fields';
      }
      if (!Array.isArray(payload.manual_products) || payload.manual_products.length === 0) {
        return 'product_fields';
      }
      for (var j = 0; j < payload.manual_products.length; j++) {
        var product = payload.manual_products[j];
        if (
          !String(product.name || '').trim() ||
          !this.isPositiveNumber(product.quantity) ||
          !this.isNonNegativeNumber(product.price) ||
          !this.isNonNegativeNumber(product.cost_price)
        ) {
          return 'product_fields';
        }
      }
    }
    return '';
  },
});
