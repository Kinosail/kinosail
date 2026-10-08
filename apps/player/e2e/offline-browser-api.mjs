// QA requests remain inside the real browser session, including Secure cookies.
const operations = ['library', 'players', 'home-assistant-on', 'home-assistant-off'];
const id = value => typeof value === 'string' && /^[a-f0-9]{16}$/.test(value);
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value);
const keys = (value, allowed, required = allowed) => object(value)
  && Object.keys(value).every(key => allowed.includes(key)) && required.every(key => Object.hasOwn(value, key));
const text = (value, maximum = 8192) => typeof value === 'string' && value.length <= maximum;
const integer = value => Number.isSafeInteger(value) && value >= 0;
const finite = value => typeof value === 'number' && Number.isFinite(value) && value >= 0;
function decode(raw) {
  const stack = []; let tokens = 0;
  for (const token of raw.matchAll(/"(?:\\.|[^"\\])*"|[{}\[\]]/g)) {
    if (++tokens > 30000) throw new Error('invalid offline API JSON');
    const value = token[0];
    if (value === '{' || value === '[') {stack.push(value === '{' ? new Set() : null); if (stack.length > 8) throw new Error('invalid offline API JSON');}
    else if (value === '}' || value === ']') stack.pop();
    else if (/^\s*:/.test(raw.slice(token.index + value.length))) {
      const current = stack.at(-1), key = JSON.parse(value);
      if (!current || current.has(key)) throw new Error('invalid offline API JSON');
      current.add(key);
    }
  }
  try {return JSON.parse(raw);} catch {throw new Error("invalid offline API JSON");}
}

function parse(raw) {
  try {return decode(raw);} catch {throw new Error('invalid offline API JSON');}
}

export async function offlineBrowserFetch(input) {
  const paths = {library: '/api/v1/library', players: '/api/v1/home-assistant/players',
    'home-assistant-on': '/api/v1/settings/home-assistant', 'home-assistant-off': '/api/v1/settings/home-assistant'};
  if (!input || typeof input !== 'object' || Array.isArray(input)
      || Object.keys(input).sort().join(',') !== 'operation,origin'
      || !Object.hasOwn(paths, input.operation) || location.origin !== input.origin) throw new Error('invalid offline API request');
  const write = input.operation.startsWith('home-assistant-');
  const init = {method: write ? 'PUT' : 'GET', credentials: 'same-origin', redirect: 'error', signal: AbortSignal.timeout(10000)};
  if (write) {
    const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
    if (typeof csrf !== 'string' || !csrf || csrf.length > 4096 || /[\x00-\x20\x7f]/.test(csrf)) throw new Error('invalid offline API CSRF');
    init.headers = {'Content-Type': 'application/json', 'X-Kinosail-CSRF': csrf};
    init.body = JSON.stringify({enabled: input.operation === 'home-assistant-on'});
  }
  const path = paths[input.operation], response = await fetch(path, init);
  if (response.status !== 200 || response.url !== input.origin + path
      || !/^application\/json(?:\s*;|$)/i.test(response.headers.get('content-type') ?? '')) throw new Error('offline API response rejected');
  const reader = response.body?.getReader(); if (!reader) throw new Error('offline API response unavailable');
  const chunks = []; let size = 0;
  try {
    while (true) {
      const {done, value} = await reader.read(); if (done) break;
      size += value.byteLength; if (size > 524288) throw new Error('offline API response too large');
      chunks.push(value);
    }
  } catch (error) {await reader.cancel().catch(() => {}); throw error;}
  finally {reader.releaseLock();}
  const bytes = new Uint8Array(size); let offset = 0;
  for (const chunk of chunks) {bytes.set(chunk, offset); offset += chunk.byteLength;}
  return new TextDecoder('utf-8', {fatal: true}).decode(bytes);
}

function catalog(data) {
  const envelope = ['items', 'view', 'sort', 'query', 'letter', 'letters', 'total', 'offset', 'limit'];
  if (!keys(data, envelope) || !Array.isArray(data.items) || data.items.length > 512
      || !['view', 'sort', 'query', 'letter'].every(key => text(data[key], 256))
      || data.letters !== null && (!Array.isArray(data.letters) || data.letters.length > 128
        || !data.letters.every(value => keys(value, ['label', 'count', 'offset']) && text(value.label, 32) && integer(value.count) && integer(value.offset)))
      || !['total', 'offset', 'limit'].every(key => integer(data[key])) || data.total < data.items.length
      || data.limit < data.items.length || data.limit < 1 || data.limit > 200
      || data.total !== data.items.length || data.offset !== 0 || !['', 'all'].includes(data.view)
      || data.sort !== 'title' || data.query !== '' || data.letter !== '') throw new Error('invalid offline catalog');
  const strings = ['sortTitle', 'year', 'plot', 'rating', 'tagline', 'genres', 'director', 'studio', 'artist', 'album', 'show', 'stream', 'artwork', 'backdrop', 'download', 'container', 'added'];
  const numbers = ['track', 'season', 'episode', 'subtitles', 'size'];
  const allowed = ['id', 'kind', 'title', 'showId', 'cast', 'progress', ...strings, ...numbers];
  const ids = new Set();
  return data.items.map(item => {
    if (!keys(item, allowed, ['id', 'kind', 'title', 'progress']) || !id(item.id) || ids.has(item.id)
        || !['video', 'audio', 'audiobook', 'book', 'photo'].includes(item.kind) || !text(item.title, 512) || !item.title
        || strings.some(key => Object.hasOwn(item, key) && !text(item[key]))
        || numbers.some(key => Object.hasOwn(item, key) && !integer(item[key]))
        || Object.hasOwn(item, 'showId') && !id(item.showId)) throw new Error('invalid offline catalog item');
    const progress = item.progress;
    if (!keys(progress, ['seconds', 'readerOffset', 'readerPage', 'watched', 'dismissed', 'updated', 'session', 'revision'], [])
        || ['seconds', 'readerOffset'].some(key => Object.hasOwn(progress, key) && !finite(progress[key]))
        || ['readerPage', 'revision'].some(key => Object.hasOwn(progress, key) && !integer(progress[key]))
        || ['watched', 'dismissed'].some(key => Object.hasOwn(progress, key) && typeof progress[key] !== 'boolean')
        || ['updated', 'session'].some(key => Object.hasOwn(progress, key) && !text(progress[key], 512))) throw new Error('invalid offline progress');
    if (Object.hasOwn(item, 'cast') && (!Array.isArray(item.cast) || item.cast.length > 128
        || item.cast.some(person => !keys(person, ['name', 'role', 'image'], ['name']) || !Object.values(person).every(value => text(value, 512))))) throw new Error('invalid offline cast');
    ids.add(item.id); return {id: item.id, kind: item.kind, title: item.title};
  });
}
function players(data) {
  if (!keys(data, ['players']) || !Array.isArray(data.players) || data.players.length > 128) throw new Error('invalid offline player response');
  const ids = new Set();
  return data.players.map(player => {
    if (!keys(player, ['id', 'name', 'state', 'title', 'itemId', 'position', 'duration', 'volume', 'muted'], ['id', 'name', 'state', 'position', 'duration', 'volume', 'muted'])
        || typeof player.id !== 'string' || !/^[A-Za-z0-9_-]{1,64}$/.test(player.id) || ids.has(player.id) || !text(player.name, 256)
        || !['playing', 'paused', 'idle', 'buffering'].includes(player.state)
        || !['position', 'duration', 'volume'].every(key => finite(player[key])) || player.volume > 1
        || typeof player.muted !== 'boolean' || Object.hasOwn(player, 'title') && !text(player.title, 512)
        || Object.hasOwn(player, 'itemId') && !id(player.itemId)) throw new Error('invalid offline player state');
    ids.add(player.id);return {itemId: player.itemId ?? null};
  });
}
export async function offlineBrowserAPI(page, operation) {
  if (!operations.includes(operation)) throw new Error('invalid offline API operation');
  const rawURL = page.url(); if (typeof rawURL !== 'string' || rawURL.length > 2048) throw new Error('invalid offline origin');
  const url = new URL(rawURL);
  if (!['http:', 'https:'].includes(url.protocol) || !['localhost', '127.0.0.1'].includes(url.hostname)
      || url.username || url.password || url.port === '0' || url.hash) throw new Error('invalid offline origin');
  const raw = await page.evaluate(offlineBrowserFetch, {operation, origin: url.origin});
  if (typeof raw !== 'string' || !raw || Buffer.byteLength(raw) > 524288) throw new Error('invalid offline API response');
  const data = parse(raw);
  if (operation === 'library') return catalog(data);
  if (operation === 'players') return players(data);
  if (!keys(data, ['status']) || data.status !== 'saved') throw new Error('invalid offline settings response');
  return {status: 200};
}
export function offlineFixture(rows, kind) {
  const titles = {video: 'Example Movie', audio: 'Example Track One', audiobook: 'Example Audiobook'};
  if (!Object.hasOwn(titles, kind) || !Array.isArray(rows) || rows.length > 512
      || rows.some(row => !keys(row, ['id', 'kind', 'title']) || !id(row.id) || !text(row.title, 512))) throw new Error('invalid offline fixture selection');
  const matches = rows.filter(row => keys(row, ['id', 'kind', 'title']) && row.kind === kind && row.title === titles[kind]);
  if (matches.length !== 1 || !id(matches[0].id)) throw new Error('offline fixture identity unavailable');
  return matches[0];
}
