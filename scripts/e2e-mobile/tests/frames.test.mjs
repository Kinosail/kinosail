import test from 'node:test';
import assert from 'node:assert/strict';
import { decodedMotion } from '../frames.mjs';
function raster(value) { return { width: 100, height: 100, data: Buffer.alloc(100 * 100 * 4, value) }; }
test('rejects a stationary poster, black playback, changed chrome, mismatched/oversized raster', () => {
  const black = raster(0), same = raster(0);
  assert.equal(decodedMotion(black, same), false);
  const chrome = raster(0); chrome.data.fill(255, 0, 100 * 10 * 4);
  assert.equal(decodedMotion(black, chrome), false);
  assert.equal(decodedMotion(black, { ...same, width: 101 }), false);
  assert.equal(decodedMotion(black, { ...same, width: 9000 }), false);
});
test('detects changing colored decoded fixture pixels in the center video region', () => {
  const first = raster(0), second = raster(0);
  for (let y = 40; y < 60; y++) for (let x = 35; x < 65; x++) {
    const p = (y * 100 + x) * 4;
    first.data[p] = 210; first.data[p + 1] = 30; first.data[p + 2] = 70;
    second.data[p] = 30; second.data[p + 1] = 220; second.data[p + 2] = 80;
  }
  assert.equal(decodedMotion(first, second), true);
});
