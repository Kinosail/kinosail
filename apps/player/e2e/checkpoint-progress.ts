import {randomUUID} from "node:crypto";
import type {Page, TestInfo} from '@playwright/test';

// PlaybackState omits zero seconds. Present values must stay numeric; coercion
// would let malformed public responses satisfy the persistence oracle.
export function checkpointSeconds(state: unknown): number {
  if (!state || typeof state !== 'object' || Array.isArray(state)) throw new TypeError('Invalid public progress state');
  const record = state as Record<string, unknown>;
  const fields = ['seconds', 'readerOffset', 'readerPage', 'watched', 'dismissed', 'updated', 'session', 'revision'];
  if (Object.keys(state).length > fields.length || Object.keys(state).some(key => !fields.includes(key))) throw new TypeError('Invalid public progress fields');
  if (JSON.stringify(state).length > 8192) throw new TypeError('Oversized public progress state');
  for (const key of ['watched', 'dismissed']) if (Object.hasOwn(state, key) && typeof record[key] !== 'boolean') throw new TypeError('Invalid public progress flag');
  for (const key of ['readerOffset', 'readerPage', 'revision']) {
    const value = record[key], maximum = key === 'readerOffset' ? 1 : key === 'readerPage' ? 10000000 : Number.MAX_SAFE_INTEGER;
    if (Object.hasOwn(state, key) && (typeof value !== 'number' || !Number.isFinite(value) || value < 0 || value > maximum || key !== 'readerOffset' && !Number.isInteger(value))) throw new TypeError('Invalid public progress number');
  }
  if (Number(record.readerOffset || 0) > 0 && !(Number(record.readerPage) > 0)) throw new TypeError('Invalid public reader position');
  if (Object.hasOwn(state, 'session') && (typeof record.session !== 'string' || new TextEncoder().encode(record.session).length > 128)) throw new TypeError('Invalid public progress session');
  if (Object.hasOwn(state, 'updated')) {
    const value = record.updated;
    if (typeof value !== 'string' || value.length > 35 || !/^\d{4}-\d{2}-\d{2}T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.test(value) || !Number.isFinite(Date.parse(value))) throw new TypeError('Invalid public progress timestamp');
    const year = Number(value.slice(0,4)), month = Number(value.slice(5,7)), day = Number(value.slice(8,10));
    const days = [31, year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0) ? 29 : 28, 31,30,31,30,31,31,30,31,30,31];
    if (month < 1 || month > 12 || day < 1 || day > days[month - 1]) throw new TypeError('Invalid public progress date');
  }
  const seconds = Object.hasOwn(state, 'seconds') ? (state as {seconds: unknown}).seconds : 0;
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds < 0 || seconds > 1000000000) throw new TypeError('Invalid public progress seconds');
  return seconds;
}

// Only independent saved-position cases prepare this baseline. The repeated
// Library durability journey retains the previous iteration's public state.
export async function prepareSavedPositionBaseline(page: Page, watch: string) {
  if (typeof watch !== 'string' || !/^\/watch\/[a-f0-9]{16}$/.test(watch)) throw new TypeError('Invalid fixture watch path');
  const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute('content');
  if (typeof csrf !== 'string' || !/^[A-Za-z0-9_-]{1,512}$/.test(csrf)) throw new TypeError('Invalid fixture CSRF');
  const path = `/api/v1/items/${watch.slice(7)}`;
  const before = await page.request.get(path);
  if (before.status() !== 200) throw new Error('Fixture progress read rejected');
  const previous = (await before.json()).item.progress;
  const beforeSeconds = checkpointSeconds(previous);
  const session = randomUUID();
  const saved = await page.request.put(path + '/progress', {
    headers: {Origin: new URL(page.url()).origin, 'X-Kinosail-CSRF': csrf},
    data: {seconds: 0, watched: false, session, revision: 1},
  });
  if (saved.status() !== 200) throw new Error('Fixture progress preparation rejected');
  const after = await page.request.get(path);
  if (after.status() !== 200) throw new Error('Fixture prepared progress read rejected');
  const accepted = (await after.json()).item.progress, afterSeconds = checkpointSeconds(accepted);
  if (afterSeconds !== 0 || accepted.watched === true || accepted.session !== session || accepted.revision !== 1) throw new Error('Fixture baseline was not accepted');
  return {beforeSeconds, afterSeconds};
}

// Diagnostic values are fixed public scalars, never session identifiers or raw
// response bodies. Missing renderer evidence remains unavailable.
export function checkpointReadWitness(state: unknown) {
  const value = state && typeof state === 'object' && !Array.isArray(state) ? state as Record<string, unknown> : {};
  const secondsPresent = Object.hasOwn(value, 'seconds'), type = value.seconds === null ? 'null' : typeof value.seconds;
  const secondsFinite = typeof value.seconds === 'number' && Number.isFinite(value.seconds) && value.seconds >= 0 && value.seconds <= 1000000000;
  return {secondsPresent, secondsType: !secondsPresent ? 'missing' : ['number','string','boolean','object','undefined','null'].includes(type) ? type : 'other',
    secondsFinite, seconds: secondsFinite ? value.seconds : 'unavailable',
    watched: typeof value.watched === 'boolean' ? value.watched : 'unavailable',
    revision: typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision >= 0 ? value.revision : 'unavailable'};
}

export async function attachCheckpointBoundary(page: Page, info: TestInfo, wire: unknown, paused: number, duration: number) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const number = (value: unknown) => typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 1000000000 ? value : 'unavailable';
  const value = wire && typeof wire === 'object' && !Array.isArray(wire) ? wire as Record<string, unknown> : {};
  const flag = (key: string) => typeof value[key] === 'boolean' ? value[key] : 'unavailable';
  const publicWire = {secondsPresent:flag('secondsPresent'), secondsType:typeof value.secondsType === 'string' && ['missing','number','string','boolean','object','undefined','null','other'].includes(value.secondsType) ? value.secondsType : 'unavailable',
    secondsFinite:flag('secondsFinite'), seconds:number(value.seconds), watched:flag('watched'), revision:typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision >= 0 ? value.revision : 'unavailable',
    bodySHA256:typeof value.bodySHA256 === 'string' && /^[a-f0-9]{64}$/.test(value.bodySHA256) ? value.bodySHA256 : 'unavailable', sessionMatches:flag('sessionMatches')};
  try {
    await Promise.race([(async () => {
      const media = await page.evaluate(() => {const video = document.querySelector('video'), start = video?.dataset.start;return {paused:video?.paused,ended:video?.ended,duration:video?.duration,currentTime:video?.currentTime,dataStart:typeof start === 'string' && /^(?:0|[1-9]\d*)(?:\.\d+)?$/.test(start) ? Number(start) : undefined};});
      await info.attach('checkpoint-baseline-boundary', {body:JSON.stringify({wire:publicWire,pausedAtRead:number(paused),duration:number(duration),media:{
        paused:typeof media.paused === 'boolean' ? media.paused : 'unavailable',ended:typeof media.ended === 'boolean' ? media.ended : 'unavailable',
        duration:number(media.duration),currentTime:number(media.currentTime),dataStart:number(media.dataStart)}}),contentType:'application/json'});
    })(), new Promise<void>(resolve => {timer = setTimeout(resolve,500);})]);
  } catch { /* Observation cannot mask the original baseline outcome. */ }
  finally {clearTimeout(timer);}
}
