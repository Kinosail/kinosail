import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const source = readFileSync(new URL('./static/downloads-progress.js', import.meta.url), 'utf8');
const snapshot = {seconds:12,watched:false,session:'offline-session',revision:1};
const job = {id:'0123456789abcdef',itemID:'movie',profileID:'viewer-a',sha256:'a'.repeat(64),transferID:'b'.repeat(32),state:'ready',readyOffline:true};
function fixture(overrides = {}) {
  const calls = [];
  const context = vm.createContext({
    addEventListener() {}, removeEventListener() {}, document:{addEventListener(){},removeEventListener(){}},
    offlineItemID:/^[A-Za-z0-9_-]{1,128}$/, offlineProfileID:/^[A-Za-z0-9_-]{1,128}$/,
    offlineTransferIDOf: value => /^[a-f0-9]{32}$/.test(value?.transferID ?? '') ? value.transferID : '',
    activeOfflineProfile: () => 'viewer-a', AbortSignal, TextDecoder, crypto: {randomUUID:()=> 'session-1'}, performance: {now:()=>6000},
    fetch: async (...args) => {calls.push(args); return new Response(JSON.stringify({item:{id:'movie',progress:snapshot},profileId:'viewer-a'}));},
    ...overrides,
  });
  vm.runInContext(source + '\nglobalThis.offlineProgressMatches = offlineProgressMatches;', context);
  return {context,calls};
}
for (const value of [null, {}, [], {...snapshot,extra:true}, {...snapshot,seconds:-1}, {...snapshot,seconds:31536001}, {...snapshot,watched:'false'}, {...snapshot,session:'a\n'}, {...snapshot,revision:1.5}, {...snapshot,revision:Number.MAX_SAFE_INTEGER+1}]) {
  test(`rejects invalid stored progress ${JSON.stringify(value)}`, () => {
    assert.throws(() => fixture().context.offlineProgress(value, true));
  });
}
test('captures a baseline only for the requested item and profile', async () => {
  const f=fixture();
  assert.equal((await f.context.downloadProgressBaseline('movie','viewer-a')).seconds, 12);
  await assert.rejects(f.context.downloadProgressBaseline('movie','viewer-b'));
  await assert.rejects(f.context.downloadProgressBaseline('other','viewer-a'));
});
test('invalid baseline identity makes no request', async () => {
  const f=fixture();
  for (const [item,profile] of [['../movie','viewer-a'],['movie',''],['movie','x'.repeat(129)]]) await assert.rejects(f.context.downloadProgressBaseline(item,profile));
  assert.equal(f.calls.length,0);
});
test('removed or replaced downloads cannot receive old progress', () => {
  const f=fixture();
  assert.ok(f.context.offlineProgressMatches({...job},job));
  for(const current of [undefined,{...job,profileID:'viewer-b'},{...job,sha256:'c'.repeat(64)},{...job,transferID:'d'.repeat(32)},{...job,state:'transferring'},{...job,readyOffline:false}]) assert.ok(!f.context.offlineProgressMatches(current,job));
});
test('restores local position and detaches progress listeners on source fallback', async () => {
  const listeners=new Map(), writes=[];
  const media={currentTime:0,duration:100,paused:true,ended:false,addEventListener:(name,fn)=>listeners.set(name,fn),removeEventListener:name=>listeners.delete(name)};
  let enabled=true;
  const f=fixture({offlineMessage:(_,fallback)=>fallback});
  f.context.updateOfflineProgress=async (_,change)=> {const current={...job,progressBaseline:{...snapshot}};change(current);writes.push(current);};
  const binding=f.context.bindOfflineProgress(media,{...job,localProgress:{...snapshot,seconds:30}},{},()=>enabled);
  listeners.get('loadedmetadata')();
  assert.equal(media.currentTime,30);
  media.currentTime=42;
  assert.equal((await binding.save(false)).ok,true);
  assert.equal(writes[0].pendingProgress.seconds,42);
  enabled=false;
  assert.equal((await binding.save(false)).ok,false);
  assert.equal(writes.length,1);
  binding.detach();
  assert.equal(listeners.size,0);
});

for (const action of ['fallback', 'exit', 'autoplay']) test(`online player restores local progress and preserves it on ${action}`, async () => {
  const streamingSource = readFileSync(new URL('../../apps/player/internal/server/static/player-streaming-offline.js', import.meta.url), 'utf8');
  const media = new EventTarget();
  Object.assign(media, {dataset:{start:'12'}, currentTime:0, currentSrc:'', src:'/stream/movie', duration:100, readyState:0, paused:true, hasAttribute:name=>name==='data-autoplay' && action==='autoplay', load(){}, getAttribute(name){return this[name];}});
  const oldResume=()=> {media.currentTime=12;};
  media.addEventListener('loadedmetadata',oldResume);
  const audioChoice={disabled:false}, audioStatus={textContent:''}, status={textContent:''};
  let cancels=0, plays=0, fallbackTime, binding;
  const writes=[];
  const f=fixture({
    player:media, cancelNetworkRecovery(){}, networkWantsPlay:action==='autoplay', destroyed:false, adaptiveGeneration:0, adaptiveStarting:false, hls:undefined,
    direct:'/stream/movie', stream:'', adaptiveActive:false, playbackTraceMethod:'direct', playbackTimelineOffset:0, playbackTimelineSeek:undefined,
    pendingResume:{seconds:12,playing:false,cancel(){cancels++;media.removeEventListener('loadedmetadata',oldResume);}},
    location:{pathname:'/watch/movie'}, qualityControl:{hidden:false}, playbackTrace(){}, showPlaybackMode(){}, requestPlay:async()=>{plays++;},
    useOriginal:(_,seconds)=>{fallbackTime=seconds;},
    document:{body:{dataset:{}},querySelector(selector){return selector==='[data-audio-track]'?audioChoice:selector==='[data-audio-status]'?audioStatus:status;},addEventListener(){},removeEventListener(){}},
    offlineMessage:(_,fallback)=>fallback,
  });
  f.context.updateOfflineProgress=async(_,change)=>{const current={...job,progressBaseline:{...snapshot}};change(current);writes.push(current);};
  f.context.window={KinosailOfflineMedia:{
    source:async()=>'/offline-media/viewer-a/0123456789abcdef',
    bindProgress:async(media,_,status)=> {binding=f.context.bindOfflineProgress(media,{...job,localProgress:{...snapshot,seconds:30}},status,()=>media.dataset.offline==='true');return binding.detach;},
    saveProgress:(media,watched)=>binding.save(watched),
    remove:async()=>{},
  }};
  vm.runInContext(streamingSource,f.context);
  await new Promise(resolve=>setImmediate(resolve));
  media.dispatchEvent(new Event('loadedmetadata'));
  media.dispatchEvent(new Event('loadeddata'));
  assert.equal(cancels,1);
  assert.equal(plays,action==='autoplay'?1:0);
  assert.equal(media.currentTime,30);
  assert.equal(audioChoice.disabled,true);
  assert.match(audioStatus.textContent,/downloaded copy/);
  media.currentTime=42;
  if (action === 'exit') {
    vm.runInContext('streaming.destroy()',f.context);
    await new Promise(resolve=>setImmediate(resolve));
    assert.equal(writes.at(-1).localProgress.seconds,42);
  } else {
  media.dispatchEvent(new Event('error'));
  await new Promise(resolve=>setImmediate(resolve));
  assert.equal(fallbackTime,42);
  assert.equal(media.dataset.offline,undefined);
  assert.equal(audioChoice.disabled,false);
  assert.equal(audioStatus.textContent,'');
  }
  media.currentTime=0;
  media.dispatchEvent(new Event('loadedmetadata'));
  assert.equal(media.currentTime,0);
});
