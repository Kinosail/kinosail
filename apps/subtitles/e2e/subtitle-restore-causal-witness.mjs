// This function is serialized into the owned page. It has no imported closure.
export function installFetchWitness(configuration) {
  if (!configuration || Object.keys(configuration).sort().join(',') !== 'item,origin' ||
    typeof configuration.origin !== 'string' || !/^https:\/\/127\.0\.0\.1:\d{1,5}$/.test(configuration.origin) ||
    typeof configuration.item !== 'string' || !/^[a-f0-9]{16}$/.test(configuration.item)) throw Error('fixed-diagnostic-boundary');
  const native = globalThis.fetch;
  const empty = () => ({ outcome: 'unreached', status: 0, signalPresent: false, signalAborted: false, pagehide: false });
  const records = { direct: empty(), captured: empty(), held: empty(), restore: empty() };
  const completion = { requestMatched: false, responseMatched: false };
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
        if (key === 'restore') completion.requestMatched = options.credentials === 'same-origin' && options.body === '{"language":"en"}';
        const record = (outcome, response) => {
          try { row.outcome = outcome; row.status = outcome === 'fulfilled' ? response.status : 0;
            if (key === 'restore' && outcome === 'fulfilled') completion.responseMatched = response.status === 204 &&
              response.url === configuration.origin + '/api/v1/subtitle-library/' + configuration.item + '/restore';
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
    read: () => ({ valid, completion: {...completion}, ...Object.fromEntries(Object.entries(records).map(([key,row]) => [key,{...row}])) }),
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

export function nativeBodylessCompletion(data) {
 const d=data?.causalDiagnostic,c=d?.completion;
 const exact=(value,keys)=>value!==null&&typeof value==='object'&&!Array.isArray(value)&&Object.keys(value).sort().join(',')===keys.slice().sort().join(',');
 if(!exact(d,['schema','valid','reason','headersEqual','framingEqual','stopped','probes','restoreNative','restoreNetwork','completion'])||
  !exact(c,['scope','caseID','browserVersion','pinnedBrowserMatched','requestMatched','responseMatched','bodyless','fixtureMatched']))return false;
 if(!d||d.schema!=='r06-causal-v2'||d.valid!==true||d.reason!=='none'||
  !['headersEqual','framingEqual','stopped'].every(k=>d[k]===true)||!c||c.scope!=='r06-native-bodyless-204-v1'||
  c.caseID!==data.caseID||!['r06-restore-headers-desktop','r06-restore-headers-phone','r06-restore-inspect-body-desktop','r06-restore-inspect-body-phone'].includes(c.caseID)||
  c.browserVersion!=='153.0.8010.12'||!['pinnedBrowserMatched','requestMatched','responseMatched','bodyless','fixtureMatched'].every(k=>c[k]===true)||
  data.protocol!=='legacy'||data.responseStatus!==204||data.prepareAttempts!==0||data.setupSaveAttempts!==1||data.restoreAttempts!==1||
  !['restoreRequestObserved','restoreResponseObserved','restoreResponseDelivered','eligible','actualRestored','historyOnce','recoverySwapped','inspectionMatches','holdEligible'].every(k=>data[k]===true)||
  !['restoreClientCancelled','holdExpired','boundaryFailed'].every(k=>data[k]===false)||data.restoreTerminal!=='request-failed'||data.restoreFailureCode!=='aborted')return false;
 const nativeShape=n=>exact(n,['outcome','status','signalPresent','signalAborted','pagehide']);
 const fulfilled=n=>nativeShape(n)&&n.outcome==='fulfilled'&&n.status===204&&n.signalPresent===false&&n.signalAborted===false&&n.pagehide===false;
 const signature=n=>exact(n,['terminal','failureCode','resourceType','cancelled','navigation'])&&n.terminal==='request-failed'&&n.failureCode==='aborted'&&n.resourceType==='Fetch'&&n.cancelled===true&&n.navigation===false;
 if(!fulfilled(d.restoreNative)||!signature(d.restoreNetwork)||!Array.isArray(d.probes)||d.probes.length!==3)return false;
 for(const [index,row]of d.probes.entries()){
  const s=row.server,n=row.native;
  if(!exact(row,['mode','native','network','server'])||!nativeShape(n)||row.mode!==['direct','captured','held'][index]||!signature(row.network)||
   !exact(s,['seen','held','delivered','cancelled','timedOut','settled'])||s.seen!==true||s.settled!==true||s.timedOut!==false)return false;
  if(index<2){if(!fulfilled(n)||s.held!==false||s.delivered!==true||s.cancelled!==false)return false;}
  else if(n?.outcome!=='rejected'||n.status!==0||n.signalPresent!==true||n.signalAborted!==true||n.pagehide!==false||
    s.held!==true||s.cancelled!==true||s.delivered!==false)return false;
 }
 return true;
}
