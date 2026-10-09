import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

test('seek URL retains audio effects after exactly one canonical offset',()=>{
  const adaptive=readFileSync(new URL('../../apps/player/internal/server/static/player-streaming-adaptive.js',import.meta.url),'utf8');
  const callback=adaptive.slice(adaptive.indexOf('const streamOffset ='),adaptive.indexOf('const useAdaptive ='));
  for(const stream of [`/hls/${id}/p/t-a0-s0-none-t0-b0-e3/index.m3u8`,`/hls/${id}/p/t-a0-s0-none-t0-b0-o30000-e3/index.m3u8`]){
    const context=vm.createContext({URL,Number,Math,stream,fullDuration:32,playbackURLBase:'http://127.0.0.1:38127'});
    vm.runInContext(`${callback};globalThis.invoke=streamAt;`,context);
    assert.equal(context.invoke(12.5).source,`/hls/${id}/p/t-a0-s0-none-t0-b0-o12500-e3/index.m3u8`);
    assert.equal(context.invoke(0).source,`/hls/${id}/p/t-a0-s0-none-t0-b0-e3/index.m3u8`);
  }
});

const source = readFileSync(new URL('../../apps/player/internal/server/static/player.js', import.meta.url), 'utf8');
const callback = source.slice(source.indexOf('const negotiateStream ='), source.indexOf('const loadHls ='));
const id = '0123456789abcdef';
function fixture(overrides = {}) {
  const calls = [];
  const player = {dataset:{playbackApi:`/api/v1/items/${id}/playback`, compatibilityMode:'remux'}};
  const result = {compatible:`/hls/${id}/p/t-a0-s0-none-t0-b0-z640x360/index.m3u8`, compatiblePlan:{allowed:true,mode:'transcode',audioIndex:0}, compatibleLabel:'Transcoding video',compatibleDescription:'Exact seeking requires video conversion.'};
  const context = vm.createContext({URL,TextDecoder,Uint8Array,AbortController,JSON,Error,Number,
    player, playbackURLBase:'http://127.0.0.1:38127', codecTypes:[['h264']], supportsCodec:async()=>true,
    adaptiveGeneration:1, destroyed:false, stream:`/hls/${id}/p/r-a0-s0-none-t0-b0/index.m3u8`,
    withPlaybackSession:value=>value,
    setTimeout:(fn, ms)=>setTimeout(fn, Math.min(ms, 30)), clearTimeout,
    fetch:async(url,options)=>{
      calls.push({url,options});
      const bytes=new TextEncoder().encode(JSON.stringify(result)); let sent=false;
      return {ok:true,url:new URL(url,'http://127.0.0.1:38127').href,headers:new Map([['content-type','application/json']]),body:{getReader:()=>({read:async()=>sent?{done:true}:(sent=true,{done:false,value:bytes}),releaseLock(){},cancel:async()=>{}})}};
    }, ...overrides});
  vm.runInContext(`${callback};globalThis.invoke=negotiateStream;globalThis.current=()=>stream;`,context);
  return {context,calls,player,result,invoke:context.invoke};
}

test('actual seek callback obtains truthful source/label before a copied window', async()=>{
  const f=fixture();
  await f.invoke(1,12.5,true);
  assert.equal(f.calls.length,1);
  const query=new URL(f.calls[0].url,'http://127.0.0.1:38127').searchParams;
  assert.equal(query.get('position'),'12.5');
  assert.equal(query.get('recipe'),'r-a0-s0-none-t0-b0');
  assert.match(f.context.current(),/\/p\/t-/);
  assert.equal(f.player.dataset.compatibilityMode,'transcode');
  assert.equal(f.player.dataset.compatibilityLabel,'Transcoding video');
});

test('initial copy negotiation and an already truthful converted seek do no fetch',async()=>{
  const copy=fixture(); await copy.invoke(1); assert.equal(copy.calls.length,0);
  const converted=fixture();converted.player.dataset.compatibilityMode='transcode';await converted.invoke(1,12.5,true);assert.equal(converted.calls.length,0);
});

test('optional codec negotiation cannot reset an already selected track',async()=>{
  const stream=`/hls/${id}/p/t-a1-s0-none-t0-b0-z640x360/index.m3u8`;
  const f=fixture({stream,codecTypes:[['av1']]});f.player.dataset.compatibilityMode='transcode';
  await f.invoke(1);
  assert.equal(f.calls.length,1);
  assert.equal(f.context.current(),stream);
});

test('invalid owned API/source/seek authority rejects without fetch or mutation',async()=>{
  for(const overrides of [{stream:'https://foreign.invalid/x'}, {stream:`/hls/${id}/p/r-a0-s0-none-t0-b0/index.m3u8#private`}, {stream:`/hls/${id}/p/${'r'.repeat(2049)}/index.m3u8`}]) {
    const f=fixture(overrides);const before=f.context.current();await assert.rejects(f.invoke(1,12.5,true));assert.equal(f.calls.length,0);assert.equal(f.context.current(),before);
  }
  for(const api of ['https://foreign.invalid/api',`/api/v1/items/${'f'.repeat(16)}/playback`, '/api/v1/items/unknown/playback']){
    const f=fixture(); f.player.dataset.playbackApi=api;await assert.rejects(f.invoke(1,12.5,true));assert.equal(f.calls.length,0);
  }
  for(const position of [-0,-1,NaN,Infinity,12.55,604801]){const f=fixture();await assert.rejects(f.invoke(1,position,true));assert.equal(f.calls.length,0);}
});

test('foreign/malformed/oversized/refused plans never replace the original source',async()=>{
  for(const change of [r=>r.compatible='https://foreign.invalid/x',r=>r.compatible=`/hls/${'f'.repeat(16)}/p/t-a0-s0-none-t0-b0/index.m3u8`,
    r=>r.compatible=`/hls/${id}/p/t-unknown/index.m3u8`,r=>r.compatible=r.compatible.replace('a0','a1'),
    r=>r.compatiblePlan.audioIndex=1,r=>r.compatiblePlan.audioIndex=true,r=>r.compatibleLabel='Remux',
    r=>r.unknown='unexpected',r=>r.compatiblePlan.unknown='unexpected',
    r=>r.compatiblePlan.allowed=false,r=>r.compatiblePlan.mode='unknown',r=>r.compatibleLabel='x'.repeat(129)]){
    const f=fixture();change(f.result);const before=f.context.current();await assert.rejects(f.invoke(1,12.5,true));assert.equal(f.context.current(),before);assert.equal(f.player.dataset.compatibilityMode,'remux');
  }
  const oversized=fixture({fetch:async()=>({ok:true,body:{getReader:()=>({read:async()=>({done:false,value:new Uint8Array(1048577)}),cancel:async()=>{},releaseLock(){}})}})});
  await assert.rejects(oversized.invoke(1,12.5,true));assert.equal(oversized.player.dataset.compatibilityMode,'remux');
});

test('stale document generation and failed reads cannot install a plan',async()=>{
  const stale=fixture();stale.context.adaptiveGeneration=2;const before=stale.context.current();await stale.invoke(1,12.5,true);assert.equal(stale.context.current(),before);assert.equal(stale.calls.length,0);
  const failure=fixture({fetch:async()=>{throw new Error('private transport')}});await assert.rejects(failure.invoke(1,12.5,true),/^Error: Compatible seek plan is unavailable$/);assert.equal(failure.player.dataset.compatibilityMode,'remux');
});

test('expired or ambiguous response never falls back to opening the old copied window',async()=>{
  const expired=fixture({fetch:async(url,{signal})=>{
    await new Promise(resolve=>signal.addEventListener('abort',resolve,{once:true}));
    return {ok:true,url,headers:new Map([['content-type','application/json']]),body:{getReader:()=>({read:async()=>({done:true}),releaseLock(){}})}};
  }});
  await assert.rejects(expired.invoke(1,12.5,true));assert.equal(expired.player.dataset.compatibilityMode,'remux');
  const duplicate=fixture({fetch:async url=>{
    const bytes=new TextEncoder().encode('{"compatiblePlan":{"allowed":false,"allowed":true,"mode":"remux"}}');let sent=false;
    return {ok:true,url,headers:new Map([['content-type','application/json']]),body:{getReader:()=>({read:async()=>sent?{done:true}:(sent=true,{done:false,value:bytes}),releaseLock(){}})}};
  }});
  await assert.rejects(duplicate.invoke(1,12.5,true));assert.equal(duplicate.player.dataset.compatibilityMode,'remux');
});

test('actual source-switch callback holds decoder mutation until planning completes',async()=>{
  const adaptive=readFileSync(new URL('../../apps/player/internal/server/static/player-streaming-adaptive.js',import.meta.url),'utf8');
  const callback=adaptive.slice(adaptive.indexOf('const useAdaptive ='),adaptive.indexOf('const startAdaptive ='));
  for(const outcome of ['accepted','stale','refused']) {
    let release;const gate=new Promise(resolve=>release=resolve),calls=[];
    class Hls {static Events={MANIFEST_PARSED:'manifest',LEVEL_SWITCHED:'level',ERROR:'error'};constructor(){calls.push('construct');}on(){}loadSource(){calls.push('load');}attachMedia(){calls.push('attach');}}
    const context=vm.createContext({Hls,player:{dataset:{compatibilityMode:'remux'},currentTime:12.5},pendingResume:undefined,
      adaptiveGeneration:1,destroyed:false,hls:{destroy:()=>calls.push('destroy')},fullDuration:32,networkWantsPlay:false,
      negotiateStream:async()=>{calls.push('plan');await gate;if(outcome==='refused')throw new Error('private rejected plan');},
      streamOffset:seconds=>seconds,streamAt:()=>({offset:12.5,source:'/qualified/index.m3u8'}),
      cancelNetworkRecovery:()=>calls.push('cancel'),resumeAfterSourceChange:()=>calls.push('resume'),playbackTrace:()=>{},showPlaybackMode:()=>{},
      showFailure:()=>calls.push('failure'),qualityControl:{hidden:true},quality:{},qualityState:{},Option:class{},
    });
    vm.runInContext(`${callback};globalThis.invoke=useAdaptive;`,context);
    const settled=Promise.resolve(context.invoke('auto',true,12.5));
    assert.deepEqual(calls,['plan']);
    if(outcome==='stale')context.adaptiveGeneration++;
    release();await settled;
    if(outcome==='accepted')assert.deepEqual(calls,['plan','cancel','resume','destroy','construct','load','attach']);
    else assert.ok(!calls.includes('destroy')&&!calls.includes('load'));
    if(outcome==='refused')assert.ok(calls.includes('failure'));
  }
});

test('actual presented-seek request selector binds truthful converted and copied routes',()=>{
  const source=readFileSync(new URL('../../apps/player/e2e/hls-presented-seek.ts',import.meta.url),'utf8');
  const callback=source.slice(source.indexOf('  const selected ='),source.indexOf('  const record =')).replace('(request: Request)','(request)');
  const context=vm.createContext({URL,origin:{origin:'http://localhost:38127'},itemID:id});
  vm.runInContext(`${callback};globalThis.invoke=selected;`,context);
  const select=url=>context.invoke({url:()=>url,method:()=> 'GET'});
  for(const [token,mode] of [['r-a0-s0-none-t0-b0','remux'],['t-a0-s0-none-t0-b0-z640x360','transcode']]) {
    const row=select(`http://localhost:38127/hls/${id}/p/${token}-o12500/360p/segment-00000.m4s`);
    assert.equal(row?.offset,12.5);assert.equal(row?.mode,mode);
  }
  for(const url of ['http://foreign.invalid/x',`http://localhost:38127/hls/${id}/p/t-a0-s0-none-t0-b0-z640x360-o12000/index.m3u8`,
    `http://localhost:38127/hls/${id}/p/t-a1-s0-none-t0-b0-z640x360-o12500/index.m3u8`, 'malformed']) assert.equal(select(url),null);
});
