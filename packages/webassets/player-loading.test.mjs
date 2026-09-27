import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

function fixture(startControl = true) {
  const listeners = new Map(), timers = new Set();
  const classes = new Set(), attrs = new Map();
  const classList = {
    contains: name => classes.has(name),
    add: name => classes.add(name),
    remove: name => classes.delete(name),
    toggle(name, on) { if (on) classes.add(name); else classes.delete(name); },
  };
  const indicator = { hidden: false };
  const status = {
    classList, dataset: { state: 'loading' }, hidden: false,
    closest: () => ({ classList }),
    querySelector: () => indicator,
    setAttribute: (name, value) => attrs.set(name, value),
    removeAttribute: name => attrs.delete(name),
  };
  const player = {
    currentTime: 12, duration: 120, paused: false, readyState: 0, dataset: {},
    buffered: { length: 1, end: () => 24 },
    addEventListener(name, handler) { listeners.set(name, handler); },
  };
  const message = { textContent: '' };
  const playerStart = startControl ? { hidden: true, addEventListener() {} } : null;
  const source = readFileSync(new URL('./static/player-status.js', import.meta.url), 'utf8');
  vm.runInNewContext(source, {
    player, playerStatus: status, playerMessage: message, playerStart,
    navigator: { userAgent: '', platform: '', maxTouchPoints: 0 },
    bufferedProgress: { setAttribute() {} }, HTMLMediaElement: { HAVE_CURRENT_DATA: 2, HAVE_FUTURE_DATA: 3 },
    setTimeout(handler) { timers.add(handler); return handler; },
    clearTimeout(handler) { timers.delete(handler); },
  });
  return {
    player, status, indicator, attrs, message, classes,
    emit: name => listeners.get(name)(),
    flush() { for (const timer of [...timers]) { timers.delete(timer); timer(); } },
  };
}

for (const event of ['pause', 'ended']) {
  test(`${event} clears visible buffering and pending buffering feedback`, () => {
    const f = fixture();
    f.emit('waiting'); f.flush();
    assert.equal(f.status.hidden, false);
    assert.equal(f.status.dataset.state, 'buffering');
    f.emit(event);
    assert.equal(f.status.hidden, true);
    assert.equal(f.attrs.has('aria-busy'), false);
    f.emit('waiting'); f.emit(event); f.flush();
    assert.equal(f.status.hidden, true);
  });
}

test('a terminal error is visible without a busy announcement or loading animation', () => {
  const f = fixture();
  f.emit('waiting'); f.emit('error'); f.flush();
  assert.equal(f.status.dataset.state, 'error');
  assert.equal(f.status.hidden, false);
  assert.equal(f.indicator.hidden, true);
  assert.equal(f.attrs.get('aria-busy'), 'false');
  assert.equal(f.classes.has('is-busy'), false);
  assert.equal(f.message.textContent, 'Playback unavailable');
  f.emit('loadstart');
  assert.equal(f.indicator.hidden, false);
  assert.equal(f.attrs.get('aria-busy'), 'true');
});

test('pause and ended preserve a recovery action or an active seek message', () => {
  const f = fixture();
  f.emit('seeking'); f.emit('pause');
  assert.equal(f.status.hidden, false);
  assert.equal(f.status.dataset.state, 'seeking');
  f.classes.add('is-recovery');
  f.status.dataset.state = 'buffering';
  f.emit('ended');
  assert.equal(f.status.hidden, false);
});

test('canplay keeps the gesture prompt until playback starts', () => {
  const f = fixture();
  f.player.paused = true;
  f.emit('kinosail:play-needs-gesture');
  f.emit('canplay');
  assert.equal(f.status.dataset.state, 'start');
  assert.equal(f.status.hidden, false);
  f.player.paused = false;
  f.emit('playing');
  assert.equal(f.status.hidden, true);
});

test('shared players without a start control keep their loading state', () => {
  const f = fixture(false);
  f.player.paused = true;
  f.emit('kinosail:play-needs-gesture');
  assert.equal(f.status.dataset.state, 'loading');
  assert.equal(f.status.hidden, false);
});
