import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

function fixture() {
  const listeners = new Map();
  const source = readFileSync(new URL('./static/pwa.js', import.meta.url), 'utf8');
  const start = source.indexOf('const loadingRequests');
  const end = source.indexOf('document.body.addEventListener("htmx:beforeRequest", (event)', start);
  vm.runInNewContext(source.slice(start, end), {
    document: { body: { addEventListener(name, listener) { listeners.set(name, listener); } } },
  });
  const attrs = new Map();
  const classes = new Set();
  const target = {
    inert: false,
    getAttribute: name => attrs.get(name) ?? null,
    setAttribute: (name, value) => attrs.set(name, value),
    removeAttribute: name => attrs.delete(name),
    matches: () => false,
    classList: { add: name => classes.add(name), remove: name => classes.delete(name) },
  };
  const emit = (name, xhr) => listeners.get(name)({ detail: { target, xhr } });
  return { target, classes, emit };
}

for (const completion of ['htmx:afterRequest', 'htmx:sendError', 'htmx:timeout', 'htmx:sendAbort']) {
  test(`${completion} restores content only after all pending requests finish`, () => {
    const { target, classes, emit } = fixture();
    const first = {}, second = {};
    emit('htmx:beforeRequest', first);
    emit('htmx:beforeRequest', second);
    assert.equal(target.inert, true);
    assert.equal(target.getAttribute('aria-busy'), 'true');
    emit(completion, first);
    emit('htmx:afterRequest', first); // Duplicate completion must not clear a newer request.
    assert.equal(classes.has('request-skeleton'), true);
    emit(completion, second);
    assert.equal(classes.size, 0);
    assert.equal(target.inert, false);
    assert.equal(target.getAttribute('aria-busy'), null);
  });
}

test('restores a pre-existing busy and inert state', () => {
  const { target, emit } = fixture();
  target.inert = true;
  target.setAttribute('aria-busy', 'false');
  const xhr = {};
  emit('htmx:beforeRequest', xhr);
  emit('htmx:afterRequest', xhr);
  assert.equal(target.inert, true);
  assert.equal(target.getAttribute('aria-busy'), 'false');
});
