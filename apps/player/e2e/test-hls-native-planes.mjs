// Generated valid media cannot exercise corrupt native plane layouts.
import test from 'node:test';
import assert from 'node:assert/strict';
import {packNative420} from './hls-native-planes.mjs';

const bytes = Uint8Array.from([99, 1, 2, 3, 4, 99, 5, 6, 7, 8, 99, 11, 12, 99, 21, 22]);
const layout = [{offset: 1, stride: 5}, {offset: 11, stride: 3}, {offset: 14, stride: 3}];
const expected = [1, 2, 3, 4, 5, 6, 7, 8, 11, 12, 21, 22];

test('I420 copies every visible byte and excludes only padding', () => {
  assert.deepEqual([...packNative420(bytes, 'I420', 4, 2, layout)], expected);
  const changed = bytes.slice(); changed[14]++;
  assert.notDeepEqual([...packNative420(changed, 'I420', 4, 2, layout)], expected);
});

test('NV12 deinterleaves all U/V bytes without reversing their order', () => {
  const interleaved = Uint8Array.from([1, 2, 3, 4, 99, 5, 6, 7, 8, 99, 11, 21, 12, 22]);
  assert.deepEqual([...packNative420(interleaved, 'NV12', 4, 2,
    [{offset: 0, stride: 5}, {offset: 10, stride: 4}])], expected);
});

test('multirow chroma padding and a sliced destination remain lossless', () => {
  const backing = new Uint8Array(48).fill(99);
  const sliced = backing.subarray(3, 43);
  sliced.set([1, 2, 3, 4], 0); sliced.set([5, 6, 7, 8], 5);
  sliced.set([9, 10, 11, 12], 10); sliced.set([13, 14, 15, 16], 15);
  sliced.set([21, 22], 22); sliced.set([23, 24], 25);
  sliced.set([31, 32], 31); sliced.set([33, 34], 34);
  assert.deepEqual([...packNative420(sliced, 'I420', 4, 4,
    [{offset: 0, stride: 5}, {offset: 22, stride: 3}, {offset: 31, stride: 3}])],
  Array.from({length: 16}, (_, n) => n + 1).concat([21, 22, 23, 24, 31, 32, 33, 34]));
});

test('truncated, aliasing, short-stride and malformed planes are rejected', () => {
  for (const bad of [[], layout.slice(0, 2), [{offset: 1, stride: 3}, ...layout.slice(1)],
    [layout[0], {offset: 7, stride: 3}, layout[2]],
    [layout[0], layout[1], {offset: 15, stride: 3}],
    [layout[0], {offset: -1, stride: 3}, layout[2]],
    [layout[0], {offset: 11.1, stride: 3}, layout[2]],
    [layout[0], {offset: 11, stride: Number.MAX_SAFE_INTEGER}, layout[2]]]) {
    assert.throws(() => packNative420(bytes, 'I420', 4, 2, bad));
  }
});

test('unsupported formats and unbounded geometry never convert or allocate', () => {
  for (const format of ['RGBA', 'I420P10', 'NV21', null]) {
    assert.throws(() => packNative420(bytes, format, 4, 2, layout));
  }
  for (const dimensions of [[0, 2], [3, 2], [4, 3], [4, Infinity], [2048, 2048], [true, 2]]) {
    assert.throws(() => packNative420(bytes, 'I420', ...dimensions, layout));
  }
  assert.throws(() => packNative420(new Uint8Array(1048577), 'I420', 4, 2, layout));
});
