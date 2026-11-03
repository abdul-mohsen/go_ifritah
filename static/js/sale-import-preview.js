// Preview of a sales import. The server checks every row before anything is posted, and this script shows each row with its status.
// An OK row, and a row whose only findings are warnings, is ticked. A duplicate is not ticked, and the user may tick it. A row with
// an error is blocked: its messages are shown, and it cannot be ticked. A row that does not look clearly valid is never OK.
(function () {
  var TEXT = {
    en: { ok: 'OK', duplicate: 'Duplicate', warning: 'Warning', error: 'Error' },
    ar: { ok: 'جاهز', duplicate: 'مكرر', warning: 'تحذير', error: 'خطأ' }
  };
  var DUPLICATE_CODES = ['duplicate_sale', 'duplicate_in_batch'];

  function language() {
    return document.documentElement.lang === 'ar' ? 'ar' : 'en';
  }

  // classify returns the status of a row: 'error', 'duplicate', 'warning' or 'ok'.
  function classify(row) {
    var errors = row.errors || [];
    if (row.valid !== true || errors.length > 0) {
      return 'error';
    }
    if (row.duplicate_of !== null && row.duplicate_of !== undefined) {
      return 'duplicate';
    }
    var warnings = row.warnings || [];
    var status = warnings.length > 0 ? 'warning' : 'ok';
    for (var warning of warnings) {
      if (DUPLICATE_CODES.includes(warning.code)) {
        status = 'duplicate';
      }
    }
    return status;
  }

  function cell(text) {
    var td = document.createElement('td');
    td.className = 'py-1 px-2';
    td.textContent = text;
    return td;
  }

  function detailsCell(row) {
    var td = document.createElement('td');
    td.className = 'py-1 px-2 text-xs';
    for (var issue of (row.errors || []).concat(row.warnings || [])) {
      var line = document.createElement('div');
      line.textContent = issue.message + ' (' + issue.code + ')';
      td.appendChild(line);
    }
    return td;
  }

  function rowElement(row, status, text) {
    var tr = document.createElement('tr');
    tr.dataset.row = String(row.row);
    tr.dataset.status = status;
    var tick = document.createElement('td');
    tick.className = 'py-1 px-2';
    var box = document.createElement('input');
    box.type = 'checkbox';
    box.dataset.rowCheck = String(row.row);
    box.checked = status === 'ok' || status === 'warning';
    box.disabled = status === 'error';
    tick.appendChild(box);
    tr.appendChild(tick);
    tr.appendChild(cell(String(row.row)));
    tr.appendChild(cell(row.customer || ''));
    tr.appendChild(cell(row.date || ''));
    tr.appendChild(cell(row.total || ''));
    tr.appendChild(cell(text[status]));
    tr.appendChild(detailsCell(row));
    return tr;
  }

  // body returns the table's body, which is where the rows go. The headers come from the page.
  function body(table) {
    return table.tBodies.length > 0 ? table.tBodies[0] : table.appendChild(document.createElement('tbody'));
  }

  // render fills the table with one line per row. It returns the count of each status.
  function render(table, rows) {
    var text = TEXT[language()];
    var counts = { ok: 0, duplicate: 0, warning: 0, error: 0 };
    var target = body(table);
    target.textContent = '';
    for (var row of rows) {
      var status = classify(row);
      counts[status] += 1;
      target.appendChild(rowElement(row, status, text));
    }
    return { counts: counts };
  }

  // clear removes the rows, and keeps the headers.
  function clear(table) {
    body(table).textContent = '';
  }

  // selectedRows returns the numbers of the rows that are ticked. A blocked row is never one of them.
  function selectedRows(table) {
    var rows = [];
    for (var box of table.querySelectorAll('input[data-row-check]')) {
      if (box.checked && !box.disabled) {
        rows.push(Number(box.dataset.rowCheck));
      }
    }
    return rows;
  }

  function summary(counts) {
    var text = TEXT[language()];
    return [
      counts.ok + ' ' + text.ok,
      counts.duplicate + ' ' + text.duplicate,
      counts.warning + ' ' + text.warning,
      counts.error + ' ' + text.error
    ].join(' · ');
  }

  // tally counts the outcomes of a commit. The backend answers a replay with the bill of the first attempt, as it answers a create.
  // Each posted row says which bill its check, run just before the post, named as a duplicate. A row that comes back with that bill
  // was replayed, which is what a retry of the same session does. Any other posted row was created.
  function tally(results) {
    var counts = { created: 0, replayed: 0, failed: 0 };
    for (var result of results) {
      if (!result.ok) {
        counts.failed += 1;
      } else if (result.duplicate_of !== null && result.duplicate_of !== undefined && result.duplicate_of === result.bill_id) {
        counts.replayed += 1;
      } else {
        counts.created += 1;
      }
    }
    return counts;
  }

  window.salePreview = { classify: classify, clear: clear, render: render, selectedRows: selectedRows, summary: summary, tally: tally };
})();
