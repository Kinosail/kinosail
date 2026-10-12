import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import {runInNewContext} from 'node:vm';

// Real FFmpeg journeys cover valid playback. They cannot produce malformed
// initialization boxes or deterministically select every Hls timing anchor.
// These isolated controls require rejection before the media consumer runs.
const source = readFileSync(new URL('../internal/server/static/player_streaming_clock.js', import.meta.url), 'utf8');
const configure = runInNewContext(source + '\n;copiedHLSClock', {Uint8Array, DataView, Map, Error});
const box = (name, ...contents) => {
  const body = Buffer.concat(contents), header = Buffer.alloc(8);
  header.writeUInt32BE(body.length + 8); header.write(name, 4);
  return Buffer.concat([header, body]);
};
function track({id = 1, scale = 90000, edit = 7500, version = 0, video = true, rate = 65536, count = 1, duplicate = false, omit = false} = {}) {
  const header = Buffer.alloc(24), media = Buffer.alloc(24), handler = Buffer.alloc(12);
  header.writeUInt32BE(id, 12); media.writeUInt32BE(scale, 12);
  handler.write(video ? 'vide' : 'soun', 8);
  const list = Buffer.alloc(version === 1 ? 28 : 20);
  list[0] = version; list.writeUInt32BE(count, 4);
  if (version === 1) list.writeBigInt64BE(BigInt(edit), 16);
  else list.writeInt32BE(edit, 12);
  list.writeUInt32BE(rate, list.length - 4);
  const edits = omit ? [] : [box('edts', box('elst', list), ...(duplicate ? [box('elst', list)] : []))];
  return box('trak', box('tkhd', header), box('mdia', box('mdhd', media), box('hdlr', handler)), ...edits);
}
function harness(offset = 60) {
  const accepted = [], errors = [], discarded = [];
  class Base {
    constructor() {
      this.context = {frag: {}};
      this.hls = {stopLoad() {}, trigger: (_, error) => errors.push(error)};
    }
    getCurrentContext() { return this.context; }
    fragContextChanged() { return this.stale; }
    _handleTransmuxComplete(value) { (this.context && !this.stale ? accepted : discarded).push(value); }
  }
  const Hls = {version: '1.7.3', DefaultConfig: {streamController: Base},
    Events: {ERROR: 'error'}, ErrorTypes: {MEDIA_ERROR: 'mediaError'}, ErrorDetails: {FRAG_PARSING_ERROR: 'fragParsingError'}};
  const Controller = configure(Hls, offset);
  return {controller: new Controller(), accepted, errors, discarded};
}
function result(bytes, {id = 1, scale = 90000, pts = 7200} = {}) {
  return {remuxResult: {initSegment: {tracks: {audiovideo: {initSegment: bytes}}, trackId: id, timescale: scale, initPTS: pts},
    video: {startPTS: 0, endPTS: 4, startDTS: -.08, endDTS: 3.92, firstKeyFramePTS: 0},
    id3: {samples: [{pts: 1, dts: .9}]}}};
}
const close = (actual, expected) => assert.ok(Math.abs(actual - expected) < 1e-9, `${actual} != ${expected}`);

test('video and later audio timing anchors preserve per-track edits', () => {
  for (const timing of [{id: 1, scale: 90000, pts: 7200}, {id: 2, scale: 48000, pts: -2560}]) {
    const run = harness(), bytes = box('moov', track(), track({id: 2, scale: 48000, edit: 1024, video: false, version: 1}));
    const value = result(bytes, timing), before = Buffer.from(bytes);
    // Pinned passthrough-remuxer normalizes raw video ticks using the selected
    // timing track's scale, which can be audio's different scale.
    value.remuxResult.video.firstKeyFramePTS = (99000 - timing.pts) / timing.scale;
    run.controller._handleTransmuxComplete(value);
    assert.equal(run.accepted.length, 1); assert.equal(run.errors.length, 0);
    const selectedEdit = timing.id === 1 ? 7500 / 90000 : 1024 / 48000;
    const delta = 60 + timing.pts / timing.scale - selectedEdit;
    close(value.remuxResult.video.startPTS, delta);
    close(value.remuxResult.video.endDTS, 3.92 + delta);
    close(value.remuxResult.video.firstKeyFramePTS, 60 + 99000 / 90000 - 7500 / 90000);
    close(value.remuxResult.id3.samples[0].pts, 1 + delta);
    assert.equal(value.remuxResult.initSegment.initPTS, -60 * timing.scale);
    assert.deepEqual(bytes, before, 'media bytes changed');
  }
});

test('reinitialization replaces the previous edit and empty flushes pass through', () => {
  const run = harness(0);
  for (const edit of [7500, 180000]) {
    const value = result(box('moov', track({edit})));
    run.controller._handleTransmuxComplete(value);
    close(value.remuxResult.video.startPTS, .08 - edit / 90000);
  }
  run.controller._handleTransmuxComplete({remuxResult: {}});
  assert.equal(run.accepted.length, 3); assert.equal(run.errors.length, 0);
});

test('init-only callbacks cache edits before the first timed samples', () => {
  const run = harness(0);
  run.controller._handleTransmuxComplete({remuxResult: {initSegment: {tracks: {audiovideo: {initSegment: box('moov', track())}}}}});
  const value = result(undefined);
  run.controller._handleTransmuxComplete(value);
  assert.equal(run.accepted.length, 2); assert.equal(run.errors.length, 0);
  close(value.remuxResult.video.startPTS, .08 - 7500 / 90000);
  run.controller._handleTransmuxComplete({remuxResult: {initSegment: {tracks: {audiovideo: {initSegment: Buffer.alloc(0)}}}}});
  assert.equal(run.accepted.length, 2); assert.equal(run.errors.length, 1);
});

test('stale and missing-context callbacks neither poison edits nor stop playback', () => {
  const run = harness(0);
  run.controller._handleTransmuxComplete(result(box('moov', track())));
  run.controller.stale = true;
  run.controller._handleTransmuxComplete(result(box('moov', track({edit: 180000}))));
  run.controller.context = null;
  run.controller._handleTransmuxComplete(result(Buffer.alloc(0)));
  run.controller.stale = false; run.controller.context = {frag: {}};
  const value = result(undefined);
  run.controller._handleTransmuxComplete(value);
  assert.equal(run.accepted.length, 2); assert.equal(run.discarded.length, 2); assert.equal(run.errors.length, 0);
  close(value.remuxResult.video.startPTS, .08 - 7500 / 90000);
});

test('an absent edit uses the unedited media clock', () => {
  const run = harness(0), value = result(box('moov', track({omit: true})));
  run.controller._handleTransmuxComplete(value);
  close(value.remuxResult.video.startPTS, .08);
  assert.equal(run.accepted.length, 1);
});

for (const [name, bytes, timing] of [
  ['empty initialization', Buffer.alloc(0)],
  ['truncated box', box('moov', track()).subarray(0, -1)],
  ['oversized initialization', Buffer.alloc(2 * 1024 * 1024 + 1)],
  ['unknown edit version', box('moov', track({version: 2}))],
  ['empty edit', box('moov', track({edit: -1}))],
  ['multiple edits', box('moov', track({count: 2}))],
  ['nonunit rate', box('moov', track({rate: 0}))],
  ['duplicate edit', box('moov', track({duplicate: true}))],
  ['duplicate track', box('moov', track(), track())],
  ['missing video', box('moov', track({video: false}))],
  ['zero scale', box('moov', track({scale: 0}))],
  ['unknown timing track', box('moov', track()), {id: 2}],
  ['nonfinite anchor', box('moov', track()), {pts: NaN}],
  ['wrong timing scale', box('moov', track()), {scale: 48000}],
]) test(`rejects ${name} before consuming media`, () => {
  for (const cached of [false, true]) {
    const run = harness(), value = result(bytes, timing), before = Buffer.from(bytes);
    if (cached) { run.controller._handleTransmuxComplete(result(box('moov', track()))); run.accepted.length = 0; }
    run.controller._handleTransmuxComplete(value);
    assert.equal(run.accepted.length, 0); assert.equal(run.errors.length, 1);
    assert.equal(run.errors[0].fatal, true);
    assert.equal(run.errors[0].error.message, 'Copied media timing is unsupported');
    assert.deepEqual(bytes, before);
  }
});

test('media without its timing initialization fails closed', () => {
  const run = harness();
  run.controller._handleTransmuxComplete({remuxResult: {video: {startPTS: 0, endPTS: 4}}});
  assert.equal(run.accepted.length, 0); assert.equal(run.errors.length, 1);
});
