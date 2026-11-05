'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { formatWorkbookDate, toRiyadhRFC3339 } = require('../static/js/purchase-import-dates.js');

test('Excel 1900 serials preserve their workbook calendar date', () => {
  assert.equal(formatWorkbookDate(46122, false), '2026-04-10');
  assert.equal(toRiyadhRFC3339(46122, false), '2026-04-10T00:00:00+03:00');
});

test('Excel 1904 serials use the workbook date system', () => {
  assert.equal(formatWorkbookDate(0, true), '1904-01-01');
});

test('the fictitious Excel leap day is rejected', () => {
  assert.equal(formatWorkbookDate(60, false), '');
});

test('ISO dates keep their workbook day while explicit timestamps use Riyadh time', () => {
  assert.equal(formatWorkbookDate('2026-04-10'), '2026-04-10');
  assert.equal(formatWorkbookDate('2026-04-09T22:30:00Z'), '2026-04-10');
  assert.equal(formatWorkbookDate(new Date('2026-04-09T21:00:00Z')), '2026-04-10');
});

test('ambiguous free-form dates are rejected rather than timezone-shifted', () => {
  assert.equal(formatWorkbookDate('04/10/2026'), '');
  assert.equal(toRiyadhRFC3339('not a date'), '');
});
