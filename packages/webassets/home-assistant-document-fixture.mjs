// Isolated fault controls: the populated Server cannot manufacture hung readers,
// malformed successful replies or exact virtual-clock lifecycle races. These do
// not establish actual tabs, authenticated Profile switching, HTTP or BFCache.
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('./static/player-home-assistant-identity.js', import.meta.url), 'utf8');
export const json = (body, status = 200, headers = {}) => new Response(JSON.stringify(body), {
  status, headers: {'Content-Type': 'application/json', ...headers},
});
export const flush = async () => {for (let index = 0; index < 50; index++) await Promise.resolve();};
export function fixture(options = {}) {
  let now = 0, nextTimer = 0, sequence = 0, profile = 'viewer-a', connected = true, sourceID = 'first';
  const timers = new Map(), calls = [], effects = [], logs = [], storage = new Map(options.storage || []);
  const events = new EventTarget(), streams = [];
  const timeout = (fn, delay, repeat = 0) => {
    const id = ++nextTimer;
    timers.set(id, {fn, time: now + delay, repeat});
    return id;
  };
  const context = vm.createContext({
    AbortController, Response, TextDecoder, JSON, performance: {now: () => now,
      getEntriesByType: () => [{type: options.navigationType || 'navigate'}]},
    addEventListener: events.addEventListener.bind(events), removeEventListener: events.removeEventListener.bind(events),
    setTimeout: timeout, clearTimeout: id => timers.delete(id),
    setInterval: (fn, delay) => timeout(fn, delay, delay), clearInterval: id => timers.delete(id),
    sessionStorage: {
      getItem(key) {if (options.deniedStorage) throw new Error('denied'); return storage.get(key) || null;},
      setItem(key, value) {if (options.deniedStorage) throw new Error('denied'); storage.set(key, value);},
    },
    console: {warn: (...values) => logs.push(values), debug: (...values) => logs.push(values)},
    EventSource: class extends EventTarget {
      constructor() {super(); this.closed = false; streams.push(this);}
      close() {this.closed = true;}
    },
    fetch: async (path, init = {}) => {
      calls.push({path, init, time: now});
      const response = await options.fetch?.(path, init, calls);
      if (response) return response;
      if (path === '/api/v1/me') return json({viewer: {id: profile}});
      if (path.endsWith('/claims')) {
        const id = JSON.parse(init.body).id || `web-${++sequence}`;
        return json({id, claim: `claim-${++sequence}-${'x'.repeat(24)}`, expiresIn: 30}, 201);
      }
      if (path.endsWith('/release')) return new Response(null, {status: 204});
      return json({command: null});
    },
  });
  // Shared origin storage and browser random/lock APIs deliberately do not exist.
  Object.defineProperty(context, 'localStorage', {get() {throw new Error('shared storage forbidden');}});
  vm.runInContext(source + '\nglobalThis.createDocumentPlayer = createHomeAssistantDocumentPlayer;', context);
  const client = context.createDocumentPlayer({
    profile: () => profile, csrf: () => 'synthetic-csrf', current: () => connected,
    diagnose: record => logs.push(JSON.parse(JSON.stringify(record))),
    snapshot: () => ({source: sourceID, body: {name: 'Fictional browser', state: 'paused', title: 'Example',
      itemId: sourceID, position: 0, duration: 12, volume: .5, muted: false}}),
    apply: async command => effects.push(JSON.parse(JSON.stringify(command))),
  });
  return {client, calls, effects, logs, storage, streams,
    setProfile: value => profile = value, changeSource: () => sourceID = 'second',
    detach: () => connected = false,
    sse(resource) {for (const stream of streams.filter(stream => !stream.closed)) {
      const event = new Event('home-assistant.command'); event.data = JSON.stringify({resource}); stream.dispatchEvent(event);
    }},
    event(name, persisted = false) {const event = new Event(name); event.persisted = persisted; events.dispatchEvent(event);},
    async tick(milliseconds) {
      const end = now + milliseconds;
      for (let steps = 0; steps < 1000; steps++) {
        const next = [...timers].filter(([, timer]) => timer.time <= end).sort((a, b) => a[1].time - b[1].time)[0];
        if (!next) break;
        now = next[1].time;
        timers.delete(next[0]);
        if (next[1].repeat) timers.set(next[0], {...next[1], time: now + next[1].repeat});
        next[1].fn();
        await flush();
        if (steps === 999) throw new Error('unbounded virtual timers');
      }
      now = end;
      await flush();
    },
  };
}
export const states = f => f.calls.filter(call => call.init.method === 'PUT');
