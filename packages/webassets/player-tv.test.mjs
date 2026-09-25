import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync(new URL('./static/player-tv.js', import.meta.url), 'utf8');
const castID = 'a'.repeat(32);
const ticket = 'b'.repeat(64);

function fixture(kind, capabilities, metadata = {}) {
  const controls = new Map();
  const control = (selector) => {
    if (!controls.has(selector)) controls.set(selector, { hidden: false, textContent: '', addEventListener(name, handler) { if (name === 'click') this.click = handler; }, append() {} });
    return controls.get(selector);
  };
  control('[data-cast]').hidden = true;
  const dialog = { querySelector: control, querySelectorAll: () => [], setAttribute() {}, removeAttribute() {} };
  const player = { tagName: kind === 'video' ? 'VIDEO' : 'AUDIO', dataset: { castApi: '/api/v1/items/123/cast', kind, artist: 'Artist', album: 'Album', ...metadata }, currentTime: 0, addEventListener() {}, pause() {} };
  let mediaInfo, castRequests = 0, loads = 0, sessionRequests = 0;
  const device = { friendlyName: 'Living room', capabilities };
  const selected = {
    getCastDevice: () => device,
    getMediaSession: () => ({ media: { contentId: `https://server.example/cast/${castID}/media?ticket=${ticket}` }, playerState: 'PLAYING', getEstimatedTime: () => 0 }),
    async loadMedia(load) { loads++; mediaInfo = load.mediaInfo; },
  };
  const castContext = { setOptions() {}, async requestSession() { sessionRequests++; }, getCurrentSession: () => selected };
  const window = { isSecureContext: true };
  class MusicTrackMediaMetadata {}
  class GenericMediaMetadata {}
  class MediaInfo { constructor(url, contentType) { this.contentId = url; this.contentType = contentType; } }
  class LoadRequest { constructor(info) { this.mediaInfo = info; } }
  vm.runInNewContext(source, {
    document: { querySelector: (selector) => selector === '[data-tv-picker]' ? dialog : null, querySelectorAll: () => [], createElement: () => ({}), head: { append() { queueMicrotask(() => window.__onGCastApiAvailable(true)); } }, body: { dataset: {} } },
    player, window, URL, location: { origin: 'https://server.example' }, csrf: '', save: async () => {}, playbackSession: '', progressRevision: 0,
    MutationObserver: class { observe() {} }, addEventListener() {}, setTimeout: () => 1, clearTimeout() {},
    cast: { framework: { CastContext: { getInstance: () => castContext } } },
    chrome: { cast: { Capability: { AUDIO_OUT: 'audio', VIDEO_OUT: 'video' }, AutoJoinPolicy: { ORIGIN_SCOPED: 'origin' }, media: { DEFAULT_MEDIA_RECEIVER_APP_ID: 'default', MusicTrackMediaMetadata, GenericMediaMetadata, MediaInfo, LoadRequest } } },
    async fetch(path, options) {
      if (options.method === 'POST') {
        castRequests++;
        return { ok: true, status: 201, text: async () => JSON.stringify({ id: castID, url: `https://server.example/cast/${castID}/media?ticket=${ticket}`, title: 'Song', contentType: 'audio/mpeg', position: 0, duration: 90, tracks: [] }) };
      }
      return { ok: true, status: 204 };
    },
  });
  return {
    async enable() { await control('[data-tv-google]').click(); },
    async choose() { await control('[data-tv-google]').click(); await control('[data-tv-google]').click(); },
    get status() { return control('[data-tv-status]').textContent; },
    get castRequests() { return castRequests; },
    get sessionRequests() { return sessionRequests; },
    get loads() { return loads; },
    get mediaInfo() { return mediaInfo; },
    MusicTrackMediaMetadata, GenericMediaMetadata,
  };
}

for (const capabilities of [undefined, 'audio', [], ['video'], ['unknown'], ['audio', 'audio'], Array(17).fill('audio')]) {
  test(`an audio receiver with invalid capabilities ${JSON.stringify(capabilities)} grants no media URL`, async () => {
    const f = fixture('audio', capabilities);
    await f.choose();
    assert.equal(f.status, 'This device cannot play audio.');
    assert.equal(f.castRequests, 0);
    assert.equal(f.loads, 0);
  });
}

test('a video title is not sent to an audio-only receiver', async () => {
  const f = fixture('video', ['audio']);
  await f.choose();
  assert.match(f.status, /cannot play video/);
  assert.equal(f.castRequests, 0);
  assert.equal(f.loads, 0);
});

test('video Cast setup directs the viewer to a display', async () => {
  const f = fixture('video', ['video']);
  await f.enable();
  assert.equal(f.status, 'Google Cast is ready. Choose a TV to play this title.');
});

for (const metadata of [{ kind: undefined }, { artist: 'x'.repeat(513) }, { album: 42 }, { artist: 'Bad\nArtist' }]) {
  test(`invalid music metadata ${JSON.stringify(metadata).slice(0, 40)} starts no Cast session`, async () => {
    const f = fixture('audio', ['audio'], metadata);
    await f.choose();
    assert.match(f.status, /invalid music details/);
    assert.equal(f.sessionRequests, 0);
    assert.equal(f.castRequests, 0);
  });
}

test('music uses Cast audio metadata on a compatible receiver', async () => {
  const f = fixture('audio', ['audio']);
  await f.choose();
  assert.equal(f.castRequests, 1, f.status);
  assert.equal(f.loads, 1);
  assert.ok(f.mediaInfo.metadata instanceof f.MusicTrackMediaMetadata);
  assert.equal(f.mediaInfo.metadata.title, 'Song');
  assert.equal(f.mediaInfo.metadata.artist, 'Artist');
  assert.equal(f.mediaInfo.metadata.albumName, 'Album');
});

test('video keeps generic Cast metadata on a display', async () => {
  const f = fixture('video', ['video']);
  await f.choose();
  assert.equal(f.castRequests, 1, f.status);
  assert.equal(f.loads, 1);
  assert.ok(f.mediaInfo.metadata instanceof f.GenericMediaMetadata);
});

test('audiobooks keep generic metadata on an audio receiver', async () => {
  const f = fixture('audiobook', ['audio']);
  await f.choose();
  assert.equal(f.castRequests, 1, f.status);
  assert.equal(f.loads, 1);
  assert.ok(f.mediaInfo.metadata instanceof f.GenericMediaMetadata);
});
