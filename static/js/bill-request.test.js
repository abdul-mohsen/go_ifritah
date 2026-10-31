const assert = require('node:assert/strict');
const test = require('node:test');

const { createController } = require('./bill-request.js');

function memoryStorage() {
  const values = new Map();
  return {
    getItem(key) {
      return values.has(key) ? values.get(key) : null;
    },
    setItem(key, value) {
      values.set(key, String(value));
    },
    removeItem(key) {
      values.delete(key);
    },
  };
}

test('persists and reuses the same bill request id after a restart', () => {
  const storage = memoryStorage();
  let generated = 0;
  const makeController = () =>
    createController({
      storage,
      generateId: () => `request-${++generated}`,
    });

  const first = makeController();
  const initial = first.prepare('pos:add-invoice');
  assert.equal(initial.id, 'request-1');
  assert.equal(first.begin('pos:add-invoice').accepted, true);
  first.finish('pos:add-invoice', 'unknown');

  const restarted = makeController();
  const recovered = restarted.prepare('pos:add-invoice');
  assert.equal(recovered.id, initial.id);
  assert.equal(recovered.status, 'unknown');
  assert.equal(restarted.begin('pos:add-invoice').accepted, true);
  restarted.finish('pos:add-invoice', 'success');

  assert.equal(storage.getItem(restarted.keyForScope('pos:add-invoice')), null);
  assert.equal(generated, 1);
});

test('rejects duplicate in-flight submits but allows a retry with the same id', () => {
  const storage = memoryStorage();
  const controller = createController({
    storage,
    generateId: () => 'request-duplicate-safe',
  });

  const first = controller.begin('pos:add-invoice');
  const duplicate = controller.begin('pos:add-invoice');

  assert.equal(first.accepted, true);
  assert.equal(duplicate.accepted, false);
  assert.equal(duplicate.reason, 'in-flight');
  assert.equal(duplicate.id, first.id);

  controller.finish('pos:add-invoice', 'unknown');
  const retry = controller.begin('pos:add-invoice');
  assert.equal(retry.accepted, true);
  assert.equal(retry.id, first.id);
});
