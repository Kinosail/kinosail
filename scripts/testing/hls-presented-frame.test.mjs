import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,symlinkSync,chmodSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {readPresentationState} from '../../apps/player/e2e/hls-presentation-state.mjs';
import {installPresentedFrames, firstPresentedFrame} from '../../apps/player/e2e/hls-presented-frame.mjs';

// Isolation failure matrix: corrupt/ambiguous pixels or dimensions; old/pending
// frames silently discarded; untrusted/change wrong value; callback overflow;
// listeners/callback survive cleanup. These are fake DOM controls, not decoding.
function peer(index=300, dimensions=[640,360]) {
 const listeners=new Map(),mediaListeners=new Map();let next,stopped=0,draws=0;
 const bits=Array.from({length:10},(_,i)=>(index>>i)&1);
 const sample=(x,y)=>{const bit=x===200?0:x===232?1:bits[Math.floor((x-24)/16)];const shade=(y===56?1-bit:bit)*255;return {data:Uint8ClampedArray.from([shade,shade,shade,255])};};
 const video={videoWidth:dimensions[0],videoHeight:dimensions[1],readyState:4,currentTime:30,paused:true,seeking:false,
  requestVideoFrameCallback(fn){next=fn;return 1;},cancelVideoFrameCallback(){stopped++;},
  getBoundingClientRect(){return {width:640,height:360};},closest(){return {classList:{contains(){return true;}}};},
  addEventListener(n,f){mediaListeners.set(n,f);},removeEventListener(n){mediaListeners.delete(n);}};
 const range={value:'12.5',matches:s=>s==='[data-player-seek]'};
 globalThis.window={};globalThis.document={readyState:'complete',querySelector:s=>s==='video'?video:null,
  addEventListener(n,f){listeners.set(n,f);},removeEventListener(n){listeners.delete(n);},
  createElement(){return {getContext(){return {drawImage(){draws++;},getImageData:sample};}};}};
 globalThis.getComputedStyle=()=>({display:'block',visibility:'visible',opacity:'1'});
 const observer=installPresentedFrames();
 return {observer,video,range,listeners,mediaListeners,frame(time=12.5){video.currentTime=time;next(100,{mediaTime:time,presentedFrames:1});},
  change(trusted=true){listeners.get('change')({isTrusted:trusted,target:range});},stats:()=>({stopped,draws})};
}
test('actual RVFC callback maps distinct target and preroll pixels, retaining pending frames',()=>{
 for(const index of [0,288,300,767]){
  const p=peer(index);p.observer.prepare();p.frame(30);p.change();p.frame();
  const s=p.observer.snapshot();assert.equal(s.frames.length,2);assert.equal(s.frames[0].afterCommit,false);
  assert.equal(s.frames[1].afterCommit,true);assert.equal(s.frames[1].pending,true);
  assert.equal(s.frames[1].frameIndex,index);assert.equal(s.failure,null);p.observer.stop();
 }
});
test('wrong/ambiguous marker is inconclusive rather than inferred from mediaTime',()=>{
 for(const index of [768,1023]){
  const p=peer(index);p.observer.prepare();p.change();p.frame();assert.equal(p.observer.snapshot().failure,'pixel-code');p.observer.stop();
 }
 const p=peer();const create=document.createElement;
 document.createElement=()=>({getContext:()=>({drawImage(){},getImageData:()=>({data:new Uint8ClampedArray([128,128,128,255])})})});
 p.observer.prepare();p.change();p.frame();assert.equal(p.observer.snapshot().failure,'pixel-code');p.observer.stop();document.createElement=create;
});
test('invalid decoded dimensions reject before drawing',()=>{
 for(const dimensions of [[0,360],[-1,360],[640,Infinity],[1281,360]]){
  const p=peer(300,dimensions);p.observer.prepare();p.change();p.frame();
  assert.equal(p.observer.snapshot().failure,'pixel-unavailable');assert.equal(p.stats().draws,0);p.observer.stop();
 }
});
test('only the trusted exact public slider change commits observation',()=>{
 const p=peer();p.observer.prepare();p.change(false);p.frame();assert.equal(p.observer.snapshot().committed,false);
 p.range.value='12.6';p.change();p.frame();assert.equal(p.observer.snapshot().committed,false);
 p.range.value='12.5';p.change();p.frame();assert.equal(p.observer.snapshot().committed,true);p.observer.stop();
});
test('callback collection is bounded and overflow remains a failed observation',()=>{
 const p=peer();p.observer.prepare();p.change();for(let i=0;i<65;i++)p.frame();
 assert.equal(p.observer.snapshot().frames.length,64);assert.equal(p.observer.snapshot().failure,'frame-overflow');p.observer.stop();
});
test('stop joins callback ownership and removes all listeners',()=>{
 const p=peer();p.observer.prepare();p.change();p.frame();p.observer.stop();
 assert.equal(p.stats().stopped,1);assert.equal(p.listeners.size,0);assert.equal(p.mediaListeners.size,0);
});
test('first visible new-source frame cannot be replaced by a later correct marker',()=>{
 const p=peer();p.observer.prepare();p.frame(30);p.change();p.frame();
 const s=p.observer.snapshot();s.frames[0].frameIndex=720;
 assert.equal(firstPresentedFrame(s).frameIndex,300);
 const preroll=structuredClone(s);preroll.frames.splice(1,0,{...s.frames[1],frameIndex:288});
 assert.throws(()=>firstPresentedFrame(preroll),/first visible/);
 const old=structuredClone(s);old.frames.splice(1,0,{...s.frames[1],frameIndex:720,pending:false});
 assert.throws(()=>firstPresentedFrame(old),/old window/);
 const held=structuredClone(s);held.frames.splice(1,0,{...s.frames[1],frameIndex:720,pending:true});
 assert.equal(firstPresentedFrame(held).frameIndex,300);
 for(const value of [null,{...s,committed:false},{...s,failure:'pixel-code'},{...s,frames:[]},
  {...s,frames:[{...s.frames[1],frameIndex:null}]}, {...s,frames:[{...s.frames[1],mediaTime:Infinity}]}]){
  assert.throws(()=>firstPresentedFrame(value));
 }
 p.observer.stop();
});
test('unknown media event fields reject before presentation acceptance',()=>{
 const p=peer();p.observer.prepare();p.change();p.frame();const s=p.observer.snapshot();
 for(const event of [{kind:'secret',afterCommit:true,currentTime:12.5},
  {kind:'playing',afterCommit:true,currentTime:-1},{kind:'playing',afterCommit:true,currentTime:12.5,raw:'not allowed'}]){
  let accepted=0;assert.throws(()=>{firstPresentedFrame({...s,events:[event]});accepted++;});assert.equal(accepted,0);
 }
 p.observer.stop();
});
test('private Owner state requires owned private parent/file before context load',()=>{
 const root=mkdtempSync(join(tmpdir(),'hls-state-')),run=join(root,'20261008T220000Z');mkdirSync(run,{mode:0o700});
 const path=join(run,'presentation-auth.json'),valid=JSON.stringify({cookies:[],origins:[]});
 try{
  writeFileSync(path,valid,{mode:0o600});assert.deepEqual(readPresentationState(run,root,'http://localhost:12345'),{cookies:[],origins:[]});
  for(const raw of ['{}','{"cookies":[],"cookies":[],"origins":[]}',JSON.stringify({cookies:[],origins:[{origin:'http://localhost:12346',localStorage:[]}]}),'x'.repeat(131073)]){
   writeFileSync(path,raw);let contexts=0;assert.throws(()=>{readPresentationState(run,root,'http://localhost:12345');contexts++;});assert.equal(contexts,0);
  }
  writeFileSync(path,valid);chmodSync(path,0o644);assert.throws(()=>readPresentationState(run,root,'http://localhost:12345'));
  rmSync(path);symlinkSync(join(root,'foreign'),path);assert.throws(()=>readPresentationState(run,root,'http://localhost:12345'));
  rmSync(path);writeFileSync(path,valid,{mode:0o600});chmodSync(run,0o755);assert.throws(()=>readPresentationState(run,root,'http://localhost:12345'));
 }finally{rmSync(root,{recursive:true,force:true});}
});
