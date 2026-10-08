// Temporary test-only endpoint observer. It forwards native fetch unchanged;
// endpoint telemetry never proves POST, keepalive, transport loss or fault success.
export function installNativeReleaseDiagnostic() {
  const prefix = 'KINOSAIL_R18_RELEASE_DIAGNOSTIC:';
  const keys = ['documents', 'pagehides', 'pageshows', 'calls', 'afterHide', 'returned',
    'response204', 'responseOther', 'rejected', 'syncThrows', 'observerErrors', 'saturated', 'corrupt'];
  const codes = ['DOCUMENT_READY', 'PAGE_HIDE', 'PAGE_SHOW', 'ENDPOINT_CALLED', 'AFTER_HIDE', 'RETURNED',
    'RESPONSE_204', 'RESPONSE_OTHER', 'REJECTED', 'SYNC_THROW', 'OBSERVER_ERROR', 'COUNTER_SATURATED', 'STATE_DISCARDED'];
  const allowedSignals = [...codes, 'TARGET_SELECTED', 'READY_COMPLETE', 'READY_INTERACTIVE', 'READY_LOADING', 'READY_UNKNOWN'];
  const emitted = Object.create(null);
  let nativeSignal = () => {};
  try {
    const receiver = console, debug = console.debug;
    nativeSignal = code => Reflect.apply(debug, receiver, ['KINOSAIL_R18_NATIVE_EVENT', code]);
  } catch (_) {}
  function signal(code) {
    try {
      if (!allowedSignals.includes(code) || (emitted[code] || 0) >= 8) return;
      emitted[code] = (emitted[code] || 0) + 1;
      nativeSignal(code);
    } catch (_) {}
  }
  let local = Array(keys.length).fill(0), denied = false, unowned = false, hidden = false, target = '';
  const blank = () => Array(keys.length).fill(0);
  function read() {
    if (denied || unowned) return local.slice();
    try {
      const value = window.name;
      if (!value) return blank();
      if (typeof value !== 'string' || !value.startsWith(prefix)) {unowned = true; return local.slice();}
      if (value.length > 512) throw new Error('invalid diagnostic state');
      const data = JSON.parse(value.slice(prefix.length));
      if (!data || Object.keys(data).sort().join(',') !== 'c,v' || data.v !== 1 ||
          !Array.isArray(data.c) || data.c.length !== keys.length ||
          !data.c.every(number => Number.isInteger(number) && number >= 0 && number <= 8)) throw new Error('invalid diagnostic state');
      return data.c.slice();
    } catch (_) {
      try {
        const value = window.name;
        if (typeof value === 'string' && value.startsWith(prefix)) {
          const reset = blank(); reset[keys.indexOf('corrupt')] = 1; return reset;
        }
      } catch (_) {}
      denied = true; return local.slice();
    }
  }
  function write(counts) {
    local = counts.slice();
    if (denied || unowned) return;
    try {window.name = prefix + JSON.stringify({v: 1, c: counts});}
    catch (_) {denied = true;}
  }
  function bump(...names) {
    try {
      const counts = read();
      for (const name of names) {
        const index = keys.indexOf(name);
        if (index < 0) continue;
        if (counts[index] < 8) counts[index]++;
        else {counts[keys.indexOf('saturated')] = 1; signal('COUNTER_SATURATED');}
        signal(codes[index]);
      }
      write(counts);
    } catch (_) {}
  }
  function matches(input) {
    if (typeof input !== 'string' || !target) return false;
    try {
      const url = new URL(input, location.href);
      return url.origin === location.origin && url.pathname === target && !url.search && !url.hash;
    } catch (_) {return false;}
  }
  window.__kinosailReleaseDiagnosticTarget = path => {
    target = typeof path === 'string' && /^\/api\/v1\/home-assistant\/players\/[A-Za-z0-9_-]{1,64}\/release$/.test(path) ? path : '';
    if (target) {
      signal('TARGET_SELECTED');
      try {
        const ready = document.readyState;
        signal(ready === 'complete' ? 'READY_COMPLETE' : ready === 'interactive' ? 'READY_INTERACTIVE'
          : ready === 'loading' ? 'READY_LOADING' : 'READY_UNKNOWN');
      } catch (_) {signal('OBSERVER_ERROR');}
    }
  };
  window.__kinosailReleaseDiagnostic = () => {
    const values = read(), counters = Object.fromEntries(keys.map((key, index) => [key, values[index]]));
    counters.pending = Math.max(0, counters.returned - counters.response204 - counters.responseOther - counters.rejected);
    counters.persistence = denied || unowned ? 0 : 1;
    const codes = [];
    if (denied) codes.push('DIAGNOSTIC_PERSISTENCE_DENIED');
    if (unowned) codes.push('DIAGNOSTIC_NAME_UNOWNED');
    if (counters.corrupt) codes.push('DIAGNOSTIC_STATE_DISCARDED');
    if (counters.saturated) codes.push('DIAGNOSTIC_COUNTER_SATURATED');
    if (counters.calls) codes.push('NATIVE_ENDPOINT_CALLED');
    if (counters.pending) codes.push('NATIVE_OUTCOME_PENDING');
    if (counters.rejected) codes.push('NATIVE_ENDPOINT_REJECTED');
    if (!counters.persistence) codes.push('CROSS_DOCUMENT_OUTCOME_UNAVAILABLE');
    return {version: 1, methodUnknown: true, counters, codes};
  };
  bump('documents');
  addEventListener('pagehide', () => {hidden = true; bump('pagehides');});
  addEventListener('pageshow', () => {hidden = false; bump('pageshows');});
  const nativeFetch = window.fetch;
  const nativeThen = Promise.prototype.then;
  const nativeStatus = Object.getOwnPropertyDescriptor(Response.prototype, 'status').get;
  window.fetch = function (...args) {
    let result;
    try {result = Reflect.apply(nativeFetch, this, args);}
    catch (error) {
      try {if (matches(args[0])) bump('calls', ...(hidden ? ['afterHide'] : []), 'syncThrows');} catch (_) {}
      throw error;
    }
    try {
      if (matches(args[0])) {
        bump('calls', ...(hidden ? ['afterHide'] : []), 'returned');
        Reflect.apply(nativeThen, result, [
          response => {
            try {bump(Reflect.apply(nativeStatus, response, []) === 204 ? 'response204' : 'responseOther');}
            catch (_) {bump('observerErrors');}
          },
          () => {bump('rejected');},
        ]);
      }
    } catch (_) {bump('observerErrors');}
    return result;
  };
}

/** Node-side receiver: private execution-context correlation, fixed aggregate fields. */
export function createDepartingConsoleCounters(getContext) {
  const fields = {DOCUMENT_READY: 'documentReady', TARGET_SELECTED: 'targetSelected', PAGE_HIDE: 'pageHide',
    PAGE_SHOW: 'pageShow', ENDPOINT_CALLED: 'endpointCalls', AFTER_HIDE: 'afterHide', RETURNED: 'returned',
    RESPONSE_204: 'response204', RESPONSE_OTHER: 'responseOther', REJECTED: 'rejected', SYNC_THROW: 'syncThrows',
    OBSERVER_ERROR: 'observerErrors', COUNTER_SATURATED: 'saturated', STATE_DISCARDED: 'stateDiscarded',
    READY_COMPLETE: 'readyComplete', READY_INTERACTIVE: 'readyInteractive', READY_LOADING: 'readyLoading',
    READY_UNKNOWN: 'readyUnknown'};
  const counters = Object.fromEntries(Object.values(fields).map(field => [field, 0]));
  return {observe(event) {
    try {
      const context = getContext(), args = event?.args;
      if (!context || event?.executionContextId !== context || !Array.isArray(args) || args.length !== 2 ||
          args[0]?.type !== 'string' || args[0]?.value !== 'KINOSAIL_R18_NATIVE_EVENT' ||
          args[1]?.type !== 'string' || typeof args[1]?.value !== 'string' || !Object.hasOwn(fields, args[1].value)) return;
      const field = fields[args[1].value];
      if (counters[field] < 8) counters[field]++; else counters.saturated = 1;
    } catch (_) {}
  }, snapshot: () => ({...counters})};
}
