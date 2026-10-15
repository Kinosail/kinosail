import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import {runInNewContext} from 'node:vm';

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
    value.remuxResult.video.firstKeyFramePTS = (99000 - timing.pts) / timing.scale;
    value.remuxResult.text = {samples: [{pts: 1, dts: .9}]};
    run.controller._handleTransmuxComplete(value);
    assert.equal(run.accepted.length, 1); assert.equal(run.errors.length, 0);
    const selectedEdit = timing.id === 1 ? 7500 / 90000 : 1024 / 48000;
    const delta = 60 + timing.pts / timing.scale - selectedEdit;
    const consumed = run.accepted[0].remuxResult;
    close(consumed.video.startPTS, delta);
    close(consumed.video.endDTS, 3.92 + delta);
    close(consumed.video.firstKeyFramePTS, 60 + 99000 / 90000 - 7500 / 90000);
    close(consumed.id3.samples[0].pts, 1 + delta);
    close(consumed.text.samples[0].pts, 1 + 60 + timing.pts / timing.scale - 7500 / 90000);
    assert.equal(consumed.initSegment.initPTS, -60 * timing.scale);
    assert.equal(value.remuxResult.initSegment.initPTS, timing.pts, 'cached callback changed');
    assert.deepEqual(bytes, before, 'media bytes changed');
  }
});

test('reinitialization replaces the previous edit and empty flushes pass through', () => {
  const run = harness(0);
  for (const edit of [7500, 180000]) {
    const value = result(box('moov', track({edit})));
    run.controller._handleTransmuxComplete(value);
    close(run.accepted.at(-1).remuxResult.video.startPTS, .08 - edit / 90000);
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
  close(run.accepted.at(-1).remuxResult.video.startPTS, .08 - 7500 / 90000);
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
  close(run.accepted.at(-1).remuxResult.video.startPTS, .08 - 7500 / 90000);
});

test('an absent edit uses the unedited media clock', () => {
  const run = harness(0), value = result(box('moov', track({omit: true})));
  run.controller._handleTransmuxComplete(value);
  close(run.accepted[0].remuxResult.video.startPTS, .08);
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
    assert.equal(run.errors[0].reason, 'copied-media-timing');
    assert.equal(run.errors[0].error.message, 'Copied media timing is unsupported');
    assert.deepEqual(bytes, before);
  }
});

test('media without its timing initialization fails closed', () => {
  const run = harness();
  run.controller._handleTransmuxComplete({remuxResult: {video: {startPTS: 0, endPTS: 4}}});
  assert.equal(run.accepted.length, 0); assert.equal(run.errors.length, 1);
});


function delayedInitialization({version = 0, delay = 130, movieScale = 1000, scale = 90000, leadingMedia = -1} = {}) {
  const original = track({scale}), at = original.indexOf('elst') + 4;
  const width = version === 1 ? 20 : 12, list = Buffer.alloc(8 + 2 * width);
  list[0] = version; list.writeUInt32BE(2, 4);
  if (version === 1) {
    list.writeBigUInt64BE(BigInt(delay), 8); list.writeBigInt64BE(BigInt(leadingMedia), 16);
    list.writeBigInt64BE(7500n, 36);
  } else {
    list.writeUInt32BE(delay, 8); list.writeInt32BE(leadingMedia, 12); list.writeInt32BE(7500, 24);
  }
  list.writeUInt32BE(65536, 8 + width - 4); list.writeUInt32BE(65536, list.length - 4);
  const replaced = box('trak', original.subarray(8, at - 16), box('edts', box('elst', list)));
  const movie = Buffer.alloc(24); movie.writeUInt32BE(movieScale, 12);
  return box('moov', box('mvhd', movie), replaced, track({id: 2, scale: 48000, edit: 1024, video: false}));
}
function fragment({id = 1, version = 1, time = 90000n, duplicate = false, missing = false} = {}) {
  const header = Buffer.alloc(8); header.writeUInt32BE(id, 4);
  const clock = Buffer.alloc(version === 0 ? 8 : 12); clock[0] = version;
  if (version === 0) clock.writeUInt32BE(Number(time), 4); else clock.writeBigUInt64BE(time, 4);
  const timing = box('tfdt', clock);
  return box('moof', box('traf', box('tfhd', header), ...(missing ? [] : [timing]), ...(duplicate ? [timing] : [])));
}
const clockValues = bytes => {
  const values = [];
  for (let at = 0; (at = bytes.indexOf('tfdt', at)) !== -1; at += 4)
    values.push(bytes[at + 4] === 1 ? bytes.readBigUInt64BE(at + 8) : BigInt(bytes.readUInt32BE(at + 8)));
  return values;
};

test('leading empty edits preserve per-track clocks and payloads across repeated callbacks', () => {
  for (const version of [0, 1]) {
    const run = harness(0), init = delayedInitialization({version});
    const bytes = Buffer.concat([fragment({version}), box('mdat', Buffer.from('unchanged-media')),
      fragment({id: 2, time: 48000n}), fragment({time: 450000n})]);
    const value = result(init), beforeInit = Buffer.from(init), beforeMedia = Buffer.from(bytes);
    value.remuxResult.video.data1 = bytes;
    for (let attempt = 0; attempt < 2; attempt++) {
      run.controller._handleTransmuxComplete(value);
      assert.equal(run.errors.length, 0); assert.equal(run.accepted.length, attempt + 1);
      const accepted = run.accepted.at(-1).remuxResult;
      close(accepted.video.startPTS, .08 - 7500 / 90000 + .13);
      assert.deepEqual(clockValues(Buffer.from(accepted.video.data1)), [101700n, 48000n, 461700n]);
      const normalized = Buffer.from(accepted.initSegment.tracks.audiovideo.initSegment);
      const at = normalized.indexOf('elst') + 4;
      assert.equal(normalized.readUInt32BE(at + 4), 1); assert.ok(normalized.includes(Buffer.from('free')));
      assert.equal(normalized.length, init.length); assert.equal(accepted.video.data1.length, bytes.length);
      assert.ok(Buffer.from(accepted.video.data1).includes(Buffer.from('unchanged-media')));
      assert.deepEqual(init, beforeInit); assert.deepEqual(bytes, beforeMedia);
      assert.equal(value.remuxResult.initSegment.initPTS, 7200);
    }
    const later = result(undefined); later.remuxResult.video.data1 = bytes;
    run.controller._handleTransmuxComplete(later);
    assert.equal(run.errors.length, 0);
    assert.deepEqual(clockValues(Buffer.from(run.accepted.at(-1).remuxResult.video.data1)), [101700n, 48000n, 461700n]);
  }
});

for (const [name, options, media, data2] of [
  ['zero movie scale', {movieScale: 0}],
  ['fractional track delay', {movieScale: 1001}],
  ['nonempty leading edit', {leadingMedia: 0}],
  ['zero empty duration', {delay: 0}],
  ['unknown media track', {}, fragment({id: 3})],
  ['duplicate media clock', {}, fragment({duplicate: true})],
  ['missing media clock', {}, fragment({missing: true})],
  ['version-zero clock overflow', {}, fragment({version: 0, time: 4294967295n})],
  ['version-one clock overflow', {}, fragment({time: 18446744073709551615n})],
  ['unknown media clock version', {}, fragment({version: 2})],
  ['truncated media box', {}, fragment().subarray(0, -1)],
  ['split passthrough payload', {}, fragment(), Buffer.from('split')],
]) test(`rejects ${name} before normalized media is consumed`, () => {
  const run = harness(0), value = result(delayedInitialization(options));
  value.remuxResult.video.data1 = media || fragment(); value.remuxResult.video.data2 = data2;
  const init = Buffer.from(value.remuxResult.initSegment.tracks.audiovideo.initSegment);
  const before = Buffer.from(value.remuxResult.video.data1);
  run.controller._handleTransmuxComplete(value);
  assert.equal(run.accepted.length, 0); assert.equal(run.errors.length, 1);
  assert.deepEqual(value.remuxResult.initSegment.tracks.audiovideo.initSegment, init);
  assert.deepEqual(value.remuxResult.video.data1, before);
});
