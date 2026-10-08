// Diagnostic only: keep the real four-second seek, network cut and assertion deadline.
export function offlineSeekMedia(input) {
  if (!input || Object.keys(input).sort().join(',') !== 'operation,origin'
      || !['arm','seek','snapshot','stop'].includes(input.operation) || location.origin !== input.origin)
    throw new Error('invalid offline media observation');
  const key = '__kinosailOfflineSeekWitness';
  if (input.operation === 'stop') {
    globalThis[key]?.stop(); delete globalThis[key]; return true;
  }
  if (input.operation !== 'arm') {
    const state = globalThis[key];
    if (!state) return null;
    return input.operation === 'seek' ? state.seek() : state.snapshot();
  }
  if (Object.hasOwn(globalThis, key)) throw new Error('offline observer already active');
  const videos = document.querySelectorAll('video');
  if (videos.length !== 1) return null;
  const video = videos[0], events = [], frames = [], started = performance.now();
  const number = value => typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 1e12 ? value : null;
  let seekRequested = false, eventOverflow = false, frameLimitReached = false, callback, stopped = false;
  const elapsed = () => Math.min(600000, Math.max(0, Math.round(performance.now()-started)));
  const kinds = ['seeking','seeked','playing','waiting','stalled','error','ended','pause','loadedmetadata','canplay','suspend'];
  const event = value => {
    if (events.length === 16) {events.shift(); eventOverflow = true;}
    events.push({kind: value.type, timeMs: elapsed(), currentTime: number(video.currentTime)});
  };
  for (const kind of kinds) video.addEventListener(kind, event);
  const schedule = () => {
    if (stopped || typeof video.requestVideoFrameCallback !== 'function' || callback !== undefined) return;
    callback = video.requestVideoFrameCallback((_, metadata) => {
      callback = undefined;
      if (frames.length < 16) frames.push({mediaTime: number(metadata.mediaTime), currentTime: number(video.currentTime),
        presentedFrames: Number.isSafeInteger(metadata.presentedFrames) && metadata.presentedFrames >= 0 ? number(metadata.presentedFrames) : null,
        pending: Boolean(document.querySelector('.media-stage')?.classList.contains('is-busy')), afterSeek: seekRequested});
      if (frames.length === 16) frameLimitReached = true;
      if (frames.length < 16 || !seekRequested) schedule();
    });
  };
  const snapshot = () => {
    const buffered = []; let rangesUnavailable = false;
    try {for (let i=0;i<Math.min(video.buffered.length,8);i++) buffered.push({start: number(video.buffered.start(i)),end: number(video.buffered.end(i))});}
    catch {rangesUnavailable = true;}
    const controller = navigator.serviceWorker?.controller;
    let workerVersion55 = false;
    try {const url = new URL(controller?.scriptURL);workerVersion55 = url.origin === input.origin && url.pathname === '/service-worker.js' && url.search === '?v=55' && !url.hash;} catch {}
    return {currentTime: number(video.currentTime), duration: number(video.duration),
      readyState: Number.isInteger(video.readyState) && video.readyState >= 0 && video.readyState <= 4 ? video.readyState : null,
      networkState: Number.isInteger(video.networkState) && video.networkState >= 0 && video.networkState <= 3 ? video.networkState : null,
      errorCode: !video.error ? 0 : Number.isInteger(video.error.code) && video.error.code >= 0 && video.error.code <= 4 ? video.error.code : null,
      paused: Boolean(video.paused), seeking: Boolean(video.seeking), buffered, rangesUnavailable,
      rvfcSupported: typeof video.requestVideoFrameCallback === 'function', events: events.slice(), frames: frames.slice(),
      seekRequested, eventOverflow, frameLimitReached, workerState: controller?.state === 'activated' ? 'activated' : controller ? 'other' : 'unavailable', workerVersion55};
  };
  globalThis[key] = {snapshot,seek:()=>{seekRequested=true;frames.length=0;frameLimitReached=false;schedule();return true;},
    stop:()=>{stopped=true;for(const kind of kinds)video.removeEventListener(kind,event);
      if(callback !== undefined && typeof video.cancelVideoFrameCallback === 'function') video.cancelVideoFrameCallback(callback);}};
  schedule(); return true;
}

export async function offlineSeekStorage(input) {
  const unavailable = {available:false,backend:'unavailable',size:null,bytes:null,chunks:null,opfsSize:null};
  if (!input || Object.keys(input).sort().join(',') !== 'jobID,origin' || location.origin !== input.origin
      || typeof input.jobID !== 'string' || !/^[a-f0-9]{16}$/.test(input.jobID)) throw new Error('invalid offline storage observation');
  if (typeof indexedDB.databases !== 'function') return unavailable;
  return new Promise(resolve => {
    let done = false, database, transaction;
    const finish = value => {if(done)return;done=true;clearTimeout(timer);
      try {transaction?.abort();} catch {} database?.close();resolve(value);};
    const timer = setTimeout(()=>finish(unavailable),1000);
    const number = value => Number.isSafeInteger(value) && value >= 0 && value <= 1e12 ? value : null;
    void (async()=>{
      try {
        const entries = await indexedDB.databases();
        if(done)return;
        if(!Array.isArray(entries) || entries.length > 64 || !entries.some(value=>value.name==='kinosail-offline-v1' && value.version===4)) return finish(unavailable);
        const request = indexedDB.open('kinosail-offline-v1');
        request.onupgradeneeded = ()=>{request.transaction.abort();finish(unavailable);};
        request.onerror = ()=>finish(unavailable);
        request.onsuccess = ()=>{
          database=request.result;if(done){database.close();return;}
          if(!database.objectStoreNames.contains('jobs') || !database.objectStoreNames.contains('chunks')) return finish(unavailable);
          try {
            transaction=database.transaction(['jobs','chunks'],'readonly');
            const store=transaction.objectStore('chunks');if(!store.indexNames.contains('jobID')) return finish(unavailable);
            const job=transaction.objectStore('jobs').get(input.jobID), count=store.index('jobID').count(input.jobID);
            transaction.onerror=()=>finish(unavailable);transaction.onabort=()=>finish(unavailable);
            transaction.oncomplete=()=>{void (async()=>{
              try {
                const record=job.result;
                if(!record || record.id!==input.jobID || !['opfs','indexeddb'].includes(record.storage)
                    || number(record.size)===null || number(record.bytes)===null || number(count.result)===null) return finish(unavailable);
                let opfsSize=null;
                try {if(record.storage==='opfs' && typeof navigator.storage.getDirectory==='function') {
                  const directory=await navigator.storage.getDirectory();if(done)return;
                  const handle=await directory.getFileHandle(input.jobID,{create:false});if(done)return;
                  opfsSize=number((await handle.getFile()).size);
                }} catch { /* Keep proven IDB metadata when the optional file-size read fails. */ }
                finish({available:true,backend:record.storage,size:record.size,bytes:record.bytes,chunks:count.result,opfsSize});
              } catch {finish(unavailable);}
            })();};
          } catch {finish(unavailable);}
        };
      } catch {finish(unavailable);}
    })();
  });
}

export function offlineSeekAction(video, origin) {
  if (typeof origin !== 'string' || origin.length > 2048 || location.origin !== origin
      || video?.tagName !== 'VIDEO' || !Number.isFinite(video.currentTime) || video.currentTime < 0)
    throw new Error('invalid offline seek action');
  try {globalThis.__kinosailOfflineSeekWitness?.seek();} catch {}
  video.currentTime = 4;
}

const object = (value, keys) => value && typeof value === 'object' && !Array.isArray(value)
  && Object.keys(value).sort().join(',') === keys.slice().sort().join(',');
const number = value => value === null || typeof value === 'number' && Number.isFinite(value) && value >= 0 && value <= 1e12;
function validMedia(value) {
  if (!object(value,['currentTime','duration','readyState','networkState','errorCode','paused','seeking','buffered','rangesUnavailable',
    'rvfcSupported','events','frames','seekRequested','eventOverflow','frameLimitReached','workerState','workerVersion55'])
      || !number(value.currentTime) || !number(value.duration) || !['activated','other','unavailable'].includes(value.workerState)) return false;
  for(const [key,max] of [['readyState',4],['networkState',3],['errorCode',4]])
    if(value[key]!==null && !(Number.isInteger(value[key]) && value[key]>=0 && value[key]<=max)) return false;
  if(!['paused','seeking','rangesUnavailable','rvfcSupported','seekRequested','eventOverflow','frameLimitReached','workerVersion55'].every(key=>typeof value[key]==='boolean')) return false;
  if(!Array.isArray(value.buffered) || value.buffered.length>8 || !value.buffered.every(range=>object(range,['start','end']) && number(range.start) && number(range.end))) return false;
  if(!Array.isArray(value.events) || value.events.length>16 || !value.events.every(event=>object(event,['kind','timeMs','currentTime'])
      && ['seeking','seeked','playing','waiting','stalled','error','ended','pause','loadedmetadata','canplay','suspend'].includes(event.kind)
      && Number.isInteger(event.timeMs) && event.timeMs>=0 && event.timeMs<=600000 && number(event.currentTime))) return false;
  return Array.isArray(value.frames) && value.frames.length<=16 && value.frames.every(frame=>object(frame,['mediaTime','currentTime','presentedFrames','pending','afterSeek'])
    && number(frame.mediaTime) && number(frame.currentTime) && (frame.presentedFrames===null || Number.isSafeInteger(frame.presentedFrames) && frame.presentedFrames>=0 && frame.presentedFrames<=1e12)
    && typeof frame.pending==='boolean' && typeof frame.afterSeek==='boolean');
}
function validStorage(value) {
  return object(value,['available','backend','size','bytes','chunks','opfsSize']) && typeof value.available==='boolean'
    && ['opfs','indexeddb','unavailable'].includes(value.backend) && ['size','bytes','chunks','opfsSize'].every(key=>number(value[key]) && (value[key]===null || Number.isSafeInteger(value[key])))
    && (value.available ? value.backend!=='unavailable' && ['size','bytes','chunks'].every(key=>value[key]!==null)
      : value.backend==='unavailable' && ['size','bytes','chunks','opfsSize'].every(key=>value[key]===null));
}
export function offlineSeekWitness(page, baseURL, jobID) {
  if(typeof baseURL!=='string' || !baseURL || baseURL.length>2048 || typeof jobID!=='string' || !/^[a-f0-9]{16}$/.test(jobID)) throw new Error('invalid offline seek fixture');
  let url;try{url=new URL(baseURL);}catch{throw new Error('invalid offline seek fixture');}
  if(!['http:','https:'].includes(url.protocol) || !['localhost','127.0.0.1'].includes(url.hostname) || url.username || url.password
      || url.port==='0' || ![url.origin,url.origin+'/'].includes(baseURL)) throw new Error('invalid offline seek fixture');
  const origin=url.origin, rows=[], requests=new WeakMap(), started=performance.now();let sequence=0, overflow=false, disconnected=false, stopped=false, setup='not_armed';
  const owned = raw => {try{
    if(typeof raw!=='string' || raw.length>2048)return false;
    const current=new URL(raw);return current.origin===origin && !current.username && !current.password && !current.hash;
  }catch{return false;}};
  if(!owned(page.url()))throw new Error('invalid current offline origin');
  const target = request => {
    try {if(!owned(request.url()))return false;const path=new URL(request.url());return !path.search
      && new RegExp('^/offline-media/[a-f0-9]{16}/'+jobID+'$').test(path.pathname) && request.method()==='GET';}catch{return false;}
  };
  const range = value => {
    if(typeof value!=='string' || value.length>80)return null;
    const match=/^bytes=(\d{1,13})-(\d{0,13})$/.exec(value);if(!match)return null;
    const start=Number(match[1]),end=match[2]?Number(match[2]):null;
    return number(start) && number(end) && (end===null || end>=start) ? {start,end} : null;
  };
  const contentRange = value => {
    if(typeof value!=='string' || value.length>80)return null;
    const match=/^bytes (\d{1,13})-(\d{1,13})\/(\d{1,13})$/.exec(value);if(!match)return null;
    const [start,end,total]=match.slice(1).map(Number);
    return [start,end,total].every(number) && end>=start && total>end ? {start,end,total} : null;
  };
  const record = (kind,request,extra={}) => {
    if(stopped || !target(request))return;
    let requestID=requests.get(request);if(!requestID){requestID=Math.min(++sequence,100000);requests.set(request,requestID);}
    if(rows.length===16){rows.shift();overflow=true;}
    let bytes=null,type='other';try{bytes=range(request.headers().range);const value=request.resourceType();if(['media','fetch','xhr'].includes(value))type=value;}catch{}
    rows.push({kind,requestID,type,timeMs:Math.min(600000,Math.max(0,Math.round(performance.now()-started))),range:bytes,...extra});
  };
  const requested=request=>record('request',request);
  const responded=response=>{
    try {const headers=response.headers(), status=response.status(), rawLength=headers['content-length'];
      record('response',response.request(),{status:Number.isInteger(status)&&status>=100&&status<=599?status:null,
        fromServiceWorker:typeof response.fromServiceWorker()==='boolean'?response.fromServiceWorker():null,
        contentRange:contentRange(headers['content-range']),length:typeof rawLength==='string'&&/^\d{1,13}$/.test(rawLength)&&number(Number(rawLength))?Number(rawLength):null});
    } catch {}
  };
  const failed=request=>{
    let family='other';try{const text=request.failure()?.errorText;
      if(typeof text==='string'&&text.length<=512)family=/ERR_ABORTED|NS_BINDING_ABORTED/.test(text)?'aborted':/NET_RESET|CONNECTION_RESET/.test(text)?'reset'
        :/INTERNET_DISCONNECTED|OFFLINE/.test(text)?'offline':/CERT|UNKNOWN_ISSUER/.test(text)?'tls':'other';
    }catch{} record('failure',request,{family});
  };
  page.on('request',requested);page.on('response',responded);page.on('requestfailed',failed);
  const deadline=Symbol('offline observation deadline');let setupReason='not_armed';
  const bounded = async callback => {let timer;try{return await Promise.race([callback(),new Promise((_,reject)=>{timer=setTimeout(()=>reject(deadline),1000);})]);}finally{clearTimeout(timer);}};
  const evaluate = operation => {if(!owned(page.url()))throw new Error('offline observation origin changed');return page.evaluate(offlineSeekMedia,{operation,origin});};
  const observe = async (callback,valid) => {
    const unavailable=()=>page.isClosed?.()===true?'page_gone':!owned(page.url())?'unowned':null;
    const before=unavailable();if(before)return {value:null,reason:before};
    try{const value=await bounded(callback),after=unavailable();if(after)return {value:null,reason:after};
      return value===null?{value:null,reason:'state_absent'}:valid(value)?{value,reason:'available'}:{value:null,reason:'schema_fail'};
    }catch(error){return {value:null,reason:unavailable()||(error===deadline?'deadline':'evaluation_failed')};}
  };
  return {
    disconnected:()=>{disconnected=true;},
    arm:async()=>{const result=await observe(()=>evaluate('arm'),value=>value===true);setup=result.value===true?'armed':'unavailable';setupReason=result.reason;},
    seek:async media=>{if(!owned(page.url()))throw new Error('offline seek origin changed');await media.evaluate(offlineSeekAction,origin);},
    attachFailure:async info=>{
      const mediaResult=await observe(()=>evaluate('snapshot'),validMedia);
      const storageResult=await observe(()=>page.evaluate(offlineSeekStorage,{jobID,origin}),validStorage);
      const media=mediaResult.value,storage=storageResult.value;
      const observations={setup:setupReason,media:mediaResult.reason,storage:storage?.available===false?'storage_unavailable':storageResult.reason};
      const body=JSON.stringify({schemaVersion:1,kind:'offline-seek',disconnected,setup,observations,media,storage,requests:rows.slice(),requestOverflow:overflow});
      if(Buffer.byteLength(body)>16384)return false;
      try{await bounded(()=>info.attach('offline-seek-failure',{contentType:'application/json',body}));return true;}catch{return false;}
    },
    stop:async()=>{if(stopped)return;stopped=true;page.off('request',requested);page.off('response',responded);page.off('requestfailed',failed);
      try{await bounded(()=>evaluate('stop'));}catch{}},
  };
}
