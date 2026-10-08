// These bounds admit the disposable SDK fixture, not arbitrary production libraries.
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value);
const keys = (value, allowed, required = allowed) => object(value)
  && Object.keys(value).every(key => allowed.includes(key)) && required.every(key => Object.hasOwn(value, key));
const text = (value, maximum) => typeof value === 'string' && Buffer.byteLength(value, 'utf8') <= maximum;
const integer = value => Number.isSafeInteger(value) && value >= 0;
const finite = (value, maximum) => typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= maximum;
const id = value => typeof value === 'string' && /^[a-f0-9]{16}$/.test(value);

// The pinned launcher supplies the allocated app.baseUrl; this checks its URL shape.
export function requireFixtureURL(value) {
  if (!text(value, 2048)) throw new Error('canonical SDK fixture URL required');
  let url;
  try {url = new URL(value);} catch {throw new Error('canonical SDK fixture URL required');}
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !/^\d{4,5}$/.test(url.port)
      || Number(url.port) < 1024 || Number(url.port) > 65535 || url.username || url.password
      || ![url.origin, url.origin + '/'].includes(value)) throw new Error('canonical SDK fixture URL required');
  return url.origin;
}

export function fixtureRequest(baseURL, path, method, body) {
  const origin = requireFixtureURL(baseURL);
  if (!text(path, 4096) || !path.startsWith('/api/v1/') || /[\x00-\x20\x7f\\#]/.test(path)
      || !['GET', 'POST', 'PUT', 'DELETE'].includes(method)) throw new Error('invalid SDK API request');
  const url = new URL(path, origin);
  if (url.origin !== origin || url.pathname !== path.split('?')[0]) throw new Error('invalid SDK API request');
  const encoded = body == null ? null : JSON.stringify(body);
  if (encoded !== null && !text(encoded, 524288) || method === 'GET' && encoded !== null) throw new Error('invalid SDK API body');
  return {origin, path, method, body: encoded};
}

// Serialized by Browser.evaluate: keep this callback independent of Node globals.
export async function fixtureBrowserFetch({origin, path, method, body}) {
  if (location.origin !== origin) throw new Error('SDK browser left its owned fixture');
  const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content ?? '';
  if (typeof csrf !== 'string' || csrf.length > 4096 || /[\x00-\x20\x7f]/.test(csrf)) throw new Error('invalid SDK CSRF');
  const response = await fetch(path, {method, credentials: 'same-origin', redirect: 'error', signal: AbortSignal.timeout(10000),
    headers: {'Content-Type': 'application/json', 'X-Kinosail-CSRF': csrf}, ...(body === null ? {} : {body})});
  if (response.url !== origin + path || response.status < 200 || response.status > 599) throw new Error('invalid SDK response origin');
  const length = response.headers.get('content-length');
  if (length !== null && (!/^\d{1,6}$/.test(length) || Number(length) > 524288)) throw new Error('SDK response too large');
  if (response.status === 204) return {status: 204, raw: ''};
  if (!/^application\/json(?:\s*;|$)/i.test(response.headers.get('content-type') ?? '')) throw new Error('SDK JSON response required');
  const reader = response.body?.getReader();
  if (!reader) throw new Error('SDK response missing');
  const chunks = []; let size = 0;
  try {
    for (;;) {
      const {done, value} = await reader.read(); if (done) break;
      size += value.byteLength; if (size > 524288) throw new Error('SDK response too large');
      chunks.push(value);
    }
  } catch (error) {await reader.cancel().catch(() => {}); throw error;}
  finally {reader.releaseLock();}
  const bytes = new Uint8Array(size); let offset = 0;
  for (const chunk of chunks) {bytes.set(chunk, offset); offset += chunk.byteLength;}
  return {status: response.status, raw: new TextDecoder('utf-8', {fatal: true}).decode(bytes)};
}

export function decodeFixtureJSON(raw) {
  if (!text(raw, 524288) || !raw) throw new Error('invalid SDK JSON');
  const stack = []; let tokens = 0;
  for (const token of raw.matchAll(/"(?:\\.|[^"\\])*"|[{}\[\]]/g)) {
    if (++tokens > 30000) throw new Error('invalid SDK JSON');
    const value = token[0];
    if (value === '{' || value === '[') {
      stack.push(value === '{' ? new Set() : null); if (stack.length > 8) throw new Error('invalid SDK JSON');
    } else if (value === '}' || value === ']') stack.pop();
    else if (/^\s*:/.test(raw.slice(token.index + value.length))) {
      const current = stack.at(-1), key = JSON.parse(value);
      if (!current || current.has(key)) throw new Error('duplicate SDK JSON key');
      current.add(key);
    }
  }
  let result;
  try {result = JSON.parse(raw);} catch {throw new Error('invalid SDK JSON');}
  function bounded(value) {
    if (typeof value === 'number' && (!Number.isFinite(value) || Number.isInteger(value) && !Number.isSafeInteger(value))) throw new Error('invalid SDK JSON number');
    if (Array.isArray(value) && value.length > 1000 || object(value) && Object.keys(value).length > 128) throw new Error('invalid SDK JSON cardinality');
    if (value !== null && typeof value === 'object') Object.values(value).forEach(bounded);
  }
  bounded(result); return result;
}

export function fixtureItem(data, title, kind) {
  const titles = {'Example Movie': 'video', 'E2E Audiobook': 'audiobook', 'E2E EPUB': 'book', 'E2E Comic': 'book', 'E2E PDF': 'book', 'E2E Photo': 'photo'};
  if (!Object.hasOwn(titles, title) || titles[title] !== kind
      || !keys(data, ['items', 'view', 'sort', 'query', 'letter', 'letters', 'total', 'offset', 'limit'])
      || !Array.isArray(data.items) || data.items.length > 200
      || !['view', 'sort', 'query', 'letter'].every(key => text(data[key], 256))
      || !['total', 'offset', 'limit'].every(key => integer(data[key])) || data.total < data.items.length
      || data.offset !== 0 || data.limit < data.items.length || data.limit < 1 || data.limit > 200
      || data.letters !== null && (!Array.isArray(data.letters) || data.letters.length > 128
        || data.letters.some(letter => !keys(letter, ['label', 'count', 'offset']) || !text(letter.label, 32) || !integer(letter.count) || !integer(letter.offset)))) throw new Error('invalid SDK fixture catalog');
  const strings = ['sortTitle', 'year', 'plot', 'rating', 'tagline', 'genres', 'director', 'studio', 'artist', 'album', 'show', 'stream', 'artwork', 'backdrop', 'download', 'container', 'added'];
  const numbers = ['track', 'season', 'episode', 'subtitles', 'size'];
  const ids = new Set();
  for (const item of data.items) {
    if (!keys(item, ['id', 'kind', 'title', 'showId', 'cast', 'progress', ...strings, ...numbers], ['id', 'kind', 'title', 'progress'])
        || !id(item.id) || ids.has(item.id) || !['video', 'audio', 'audiobook', 'book', 'photo'].includes(item.kind)
        || !text(item.title, 512) || !item.title || strings.some(key => Object.hasOwn(item, key) && !text(item[key], 8192))
        || numbers.some(key => Object.hasOwn(item, key) && !integer(item[key]))
        || Object.hasOwn(item, 'showId') && !id(item.showId)) throw new Error('invalid SDK fixture item');
    const progress = item.progress;
    if (!keys(progress, ['seconds', 'readerOffset', 'readerPage', 'watched', 'dismissed', 'updated', 'session', 'revision'], [])
        || ['seconds', 'readerOffset'].some(key => Object.hasOwn(progress, key) && !finite(progress[key], key === 'seconds' ? 1e9 : 1))
        || ['readerPage', 'revision'].some(key => Object.hasOwn(progress, key) && !integer(progress[key]))
        || progress.readerPage > 10000000 || progress.readerOffset > 0 && !(progress.readerPage > 0)
        || ['watched', 'dismissed'].some(key => Object.hasOwn(progress, key) && typeof progress[key] !== 'boolean')
        || Object.hasOwn(progress, 'updated') && !text(progress.updated, 512)
        || Object.hasOwn(progress, 'session') && !text(progress.session, 128)) throw new Error('invalid SDK fixture progress');
    if (Object.hasOwn(item, 'cast') && (!Array.isArray(item.cast) || item.cast.length > 128
        || item.cast.some(person => !keys(person, ['name', 'role', 'image'], ['name']) || !Object.values(person).every(value => text(value, 512))))) throw new Error('invalid SDK fixture cast');
    ids.add(item.id);
  }
  const matches = data.items.filter(item => item.title === title && item.kind === kind);
  if (matches.length !== 1) throw new Error('unique SDK fixture missing');
  return matches[0];
}
