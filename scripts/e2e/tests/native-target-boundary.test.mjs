import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import {join} from 'node:path';
import * as boundary from '../fixture-response.mjs';
import * as setupBoundary from '../fixture-setup.mjs';
const held='9191a5dbbbc8459895b18552986f43a33d5f53fc';
const source=path=>existsSync(path)?readFileSync(path,'utf8'):execFileSync('git',['show',held+':'+path],{encoding:'utf8',maxBuffer:131072});
const base='http://127.0.0.1:18769',movie={id:'0123456789abcdef',kind:'video',title:'Example Movie',progress:{}};
const catalog=items=>({items,view:'movies',sort:'title',query:'',letter:'',letters:null,total:items.length,offset:0,limit:200});
function peer(platform,data,playback) {
 let callback;const run={identity:{platform:'ios',port:'18769'}};const effects={itemRequests:0,native:0,approval:0};
 const admin={read:async path=>{if(path==='/api/v1/library?view=movies')return data;effects.itemRequests++;if(playback!==undefined)return playback;throw Error('controlled valid target request');},approve:async()=>{effects.approval++;}};
 const expect=value=>({toBe:expected=>assert.equal(value,expected)});
 expect.poll=fn=>({toBe:async expected=>assert.equal(await fn(),expected)});
 const code=stripTypeScriptTypes(source(`scripts/e2e-${platform==='phone'?'mobile':'tv'}/tests/${platform}.e2e.ts`)).replace(/^import .*;\n/gm,'');
 runInNewContext(code,{test:(name,fn)=>{callback=fn;},expect,...boundary,...setupBoundary,readControl:()=>run,
  owner:async()=>admin,join,process:{cwd:()=>'/owned/native'},AbortSignal,
  enterServerAddress:async()=>{effects.native++;},focusAndSelect:async()=>{effects.native++;}});
 return {effects,run:()=>callback({platform:'ios',app:{open:async()=>{effects.native++;}},device:{installApp:async()=>{effects.native++;}},tv:{client:{},selection:{platform:'ios'},run}})};
}
for(const platform of ['phone','tv']) {
 for(const [name,data] of [
  ['missing',{}],['wrong type',{items:{}}],['unknown field',{...catalog([movie]),extra:true}],
  ['noncanonical ID',catalog([{...movie,id:'Movie_123'}])],['uppercase ID',catalog([{...movie,id:movie.id.toUpperCase()}])],
  ['unsafe ID',catalog([{...movie,id:'../foreign'}])],['duplicate title',catalog([movie,{...movie,id:'1111111111111111'}])],
  ['duplicate unrelated ID',catalog([movie,{...movie,title:'Other'}])],['unsafe unrelated ID',catalog([movie,{...movie,title:'Other',id:'foreign'}])],
  ['unknown kind',catalog([{...movie,kind:'unknown'}])],['unknown item field',catalog([{...movie,extra:true}])],
  ['oversized',catalog([movie,...Array.from({length:200},(_,i)=>({...movie,title:'Other'+i,id:i.toString(16).padStart(16,'0')}))])],
  ['invalid progress',catalog([{...movie,progress:{seconds:1e400}}])]
 ]) test(`${platform} registered journey rejects ${name} before item/pairing/native actions`,async()=>{
  const p=peer(platform,data);await assert.rejects(p.run());assert.deepEqual(p.effects,{itemRequests:0,native:0,approval:0});
 });
 test(`${platform} registered journey accepts only the unique canonical fixture before its target request`,async()=>{
  const p=peer(platform,catalog([movie]));await assert.rejects(p.run(),/controlled valid target request/);
  assert.deepEqual(p.effects,{itemRequests:1,native:0,approval:0});
 });
}
function reader(raw) {
 const code=source('scripts/e2e-mobile/owner.mjs').replace(/^import .*;\n/gm,'').replace(/^export /gm,'');
 const response=new Response(raw,{status:200,headers:{'Content-Type':'application/json'}});Object.defineProperty(response,'url',{value:base+'/api/v1/library'});
 return runInNewContext(`(()=>{${code};return request;})()`,{Buffer,...boundary,...setupBoundary,fetch:async()=>response,URL,AbortSignal,TextDecoder,Uint8Array});
}
for(const [name,raw] of [['duplicate','{"items":[],"items":[]}'],['escaped duplicate','{"items":[],"\\u0069tems":[]}'],
 ['invalid UTF8',Buffer.concat([Buffer.from('{"name":"'),Buffer.from([0xc3,0x28]),Buffer.from('"}')])],['nonfinite','{"value":1e400}'],['unsafe integer','{"value":9007199254740993}'],
 ['depth','['.repeat(10)+'0'+']'.repeat(10)],['cardinality',JSON.stringify(Array(1001).fill(0))]])
 test(`native API reader rejects ${name} before caller actions`,async()=>{let downstream=0;await assert.rejects(async()=>{await reader(raw)(base,'/api/v1/library');downstream++;});assert.equal(downstream,0);});

for(const duration of [18,604801]) test(`phone generated fixture duration ${duration} rejects before native actions`,async()=>{const p=peer('phone',catalog([movie]),{duration});await assert.rejects(p.run());assert.deepEqual(p.effects,{itemRequests:1,native:0,approval:0});});
