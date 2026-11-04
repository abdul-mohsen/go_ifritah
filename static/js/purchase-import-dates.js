(function (root) {
  var DAY_MS = 24 * 60 * 60 * 1000;
  var RIYADH_OFFSET_MS = 3 * 60 * 60 * 1000;

  function dateString(year, month, day) {
    if (!Number.isInteger(year) || !Number.isInteger(month) || !Number.isInteger(day)) return '';
    if (month < 1 || month > 12 || day < 1 || day > 31) return '';

    var check = new Date(0);
    check.setUTCHours(0, 0, 0, 0);
    check.setUTCFullYear(year, month - 1, day);
    if (check.getUTCFullYear() !== year || check.getUTCMonth() !== month - 1 || check.getUTCDate() !== day) return '';
    return String(year).padStart(4, '0') + '-' + String(month).padStart(2, '0') + '-' + String(day).padStart(2, '0');
  }

  function dateFromTimestamp(timestamp) {
    if (!Number.isFinite(timestamp)) return '';
    var date = new Date(timestamp + RIYADH_OFFSET_MS);
    return dateString(date.getUTCFullYear(), date.getUTCMonth() + 1, date.getUTCDate());
  }

  function excelSerialDate(serial, date1904) {
    if (!Number.isFinite(serial) || serial < 0) return '';
    var days = Math.floor(serial);
    if (date1904) {
      return dateFromTimestamp(Date.UTC(1904, 0, 1) + days * DAY_MS);
    }
    if (days === 60) return '';
    if (days > 60) days -= 1;
    return dateFromTimestamp(Date.UTC(1899, 11, 31) + days * DAY_MS);
  }

  function formatWorkbookDate(value, date1904) {
    if (value instanceof Date) return dateFromTimestamp(value.getTime());
    if (typeof value === 'number') return excelSerialDate(value, Boolean(date1904));

    var text = String(value == null ? '' : value).trim();
    if (!text) return '';

    var match =
      /^(\d{4})[-/](\d{1,2})[-/](\d{1,2})(?:[ T](\d{2}):(\d{2})(?::(\d{2})(?:\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?$/.exec(
        text
      );
    if (!match) return '';

    var year = Number(match[1]);
    var month = Number(match[2]);
    var day = Number(match[3]);
    var formatted = dateString(year, month, day);
    if (!formatted) return '';

    if (match[4] !== undefined) {
      var hour = Number(match[4]);
      var minute = Number(match[5]);
      var second = Number(match[6] || 0);
      if (hour > 23 || minute > 59 || second > 59) return '';
    }

    if (match[7]) {
      var timestamp = Date.parse(text);
      return Number.isNaN(timestamp) ? '' : dateFromTimestamp(timestamp);
    }
    return formatted;
  }

  function toRiyadhRFC3339(value, date1904) {
    var date = formatWorkbookDate(value, date1904);
    return date ? date + 'T00:00:00+03:00' : '';
  }

  var api = {
    formatWorkbookDate: formatWorkbookDate,
    toRiyadhRFC3339: toRiyadhRFC3339,
  };
  root.AfritaPurchaseImportDates = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
})(typeof window !== 'undefined' ? window : globalThis);
