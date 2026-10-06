// This function is serialized into the owned page. It has no imported closure.
export function installFetchWitness(configuration) {
  if (!configuration || Object.keys(configuration).sort().join(',') !== 'item,origin' ||
    typeof configuration.origin !== 'string' || !/^https:\/\/127\.0\.0\.1:\d{1,5}$/.test(configuration.origin) ||
    typeof configuration.item !== 'string' || !/^[a-f0-9]{16}$/.test(configuration.item)) throw Error('fixed-diagnostic-boundary');
  const native = globalThis.fetch;
  const empty = () => ({ outcome: 'unreached', status: 0, signalPresent: false, signalAborted: false, pagehide: false });
  const records = { direct: empty(), captured: empty(), held: empty(), restore: empty() };
  let valid = true, hidden = false, heldController;
  const onHide = () => { hidden = true; for (const row of Object.values(records)) if (row.outcome === 'pending') row.pagehide = true; };
  globalThis.addEventListener('pagehide', onHide);
  const wrapper = function (...args) {
    const promise = native.apply(this, args); // Preserve the exact native promise and caller arguments.
    try {
      const [input, options] = args;
      let key;
      if (typeof input === 'string' && input.length <= 2048 && options?.method === 'POST') {
        const observedURL = new URL(input, configuration.origin).href;
        if (observedURL === configuration.origin + '/api/v1/subtitle-library/' + configuration.item + '/restore') key = 'restore';
        if (observedURL === configuration.origin + '/__r06_restore/probe' && typeof options.body === 'string' && options.body.length <= 256) {
          const parsed = JSON.parse(options.body);
          if (Object.keys(parsed).join(',') === 'mode' && ['direct', 'captured', 'held'].includes(parsed.mode)) key = parsed.mode;
        }
      }
      if (key) {
        const row = records[key];
        if (row.outcome !== 'unreached') { valid = false; return promise; }
        row.outcome = 'pending'; row.signalPresent = !!options.signal; row.pagehide = hidden;
        const record = (outcome, response) => {
          try { row.outcome = outcome; row.status = outcome === 'fulfilled' ? response.status : 0;
            row.signalAborted = !!options.signal?.aborted; row.pagehide = hidden; } catch { valid = false; }
        };
        // Observe fulfillment/rejection only. Never read/replace a Response or body.
        void promise.then(response => record('fulfilled', response), () => record('rejected'));
      }
    } catch { valid = false; }
    return promise;
  };
  globalThis.fetch = wrapper;
  const witness = {
    read: () => ({ valid, ...Object.fromEntries(Object.entries(records).map(([key,row]) => [key,{...row}])) }),
    probe: mode => {
      if (!['direct','captured','held'].includes(mode)) throw Error('fixed-diagnostic-boundary');
      if (mode === 'held') heldController = new AbortController();
      return globalThis.fetch(configuration.origin + '/__r06_restore/probe', {
        method:'POST',credentials:'same-origin',headers:{'Content-Type':'application/json'},
        body:JSON.stringify({mode}),...(mode==='held'?{signal:heldController.signal}:{}) });
    },
    cancelHeld: () => { if (!heldController) throw Error('fixed-diagnostic-boundary'); heldController.abort(); },
    close: () => { if (globalThis.fetch === wrapper) globalThis.fetch = native; globalThis.removeEventListener('pagehide',onHide); },
  };
  globalThis.__r06CausalWitness = witness;
  return witness;
}

export function equalDiagnosticHeaders(first,second) {
 return Array.isArray(first)&&Array.isArray(second)&&first.length===second.length&&first.length<=6&&
   first.every((value,index)=> (value===null||typeof value==='string'&&value.length<=128)&&value===second[index]);
}
