import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const script = readFileSync(new URL('./static/pwa-navigation.js', import.meta.url), 'utf8');
const source = script.slice(script.indexOf('const syncCompactMenu'), script.indexOf('const editable'));
function fixture(desktop) {
  const documentListeners = new Map(), windowListeners = new Map(), mediaListeners = new Map();
  function menu() {
    return { open: true, focused: false, listeners: new Map(),
      removeAttribute() { this.open = false; }, contains(target) { return target === this; },
      querySelector() { return { focus: () => { this.focused = true; } }; },
      addEventListener(name, handler) { this.listeners.set(name, handler); },
    };
  }
  const compactMenu = menu(), navigationMore = menu();
  vm.runInNewContext(source, {
    compactMenu, navigationMore, animateMotion() {},
    desktopMenu: { matches: desktop, addEventListener: (name, handler) => mediaListeners.set(name, handler) },
    document: { addEventListener: (name, handler) => documentListeners.set(name, handler) },
    window: { addEventListener: (name, handler) => windowListeners.set(name, handler) },
  });
  return { compactMenu, navigationMore, documentListeners, windowListeners, mediaListeners };
}
for (const desktop of [true, false]) {
  test(`menus dismiss and restore focus on ${desktop ? 'desktop' : 'mobile'}`, () => {
    const f = fixture(desktop);
    assert.equal(f.compactMenu.open, false);
    f.compactMenu.open = true;
    f.compactMenu.listeners.get('toggle')();
    assert.equal(f.navigationMore.open, false);
    f.windowListeners.get('keydown')({ key: 'Escape' });
    assert.equal(f.compactMenu.open, false);
    assert.equal(f.compactMenu.focused, true);
    f.navigationMore.open = true;
    f.windowListeners.get('keydown')({ key: 'Escape' });
    assert.equal(f.navigationMore.open, false);
    assert.equal(f.navigationMore.focused, true);
    f.compactMenu.open = true;
    f.documentListeners.get('click')({ target: {} });
    assert.equal(f.compactMenu.open, false);
    f.compactMenu.open = true;
    f.mediaListeners.get('change')();
    assert.equal(f.compactMenu.open, false);
  });
}
