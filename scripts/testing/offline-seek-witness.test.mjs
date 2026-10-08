import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {runInNewContext} from 'node:vm';
import {offlineSeekWitness,offlineSeekMedia,offlineSeekStorage,offlineSeekAction} from '../../apps/player/e2e/offline-seek-witness.mjs';
const origin='http://127.0.0.1:38127',jobID='0123456789abcdef';
test('offline observer surrounds the same real seek and cleans up',()=>{
 const source=readFileSync(new URL('../../apps/player/e2e/test-instance-production.spec.ts',import.meta.url),'utf8');
 const block=source.slice(source.indexOf('test("a real offline download plays'),source.indexOf('test("video and music'));
 for(const binding of ['offlineSeekWitness(page, baseURL, id)','await witness.arm()','await witness.seek(media)',
  'await witness.attachFailure(testInfo)','await witness.stop()','!video.seeking && video.currentTime >= 4'])assert.ok(block.includes(binding));
});
function mediaPeer(){
 const listeners=new Map(),callbacks=new Map(),events=[];let counter=0;
 const video={currentTime:0,duration:12,readyState:4,networkState:1,error:null,paused:false,seeking:false,
  buffered:{length:1,start:()=>0,end:()=>12},addEventListener:(kind,fn)=>listeners.set(kind,fn),
  removeEventListener:(kind,fn)=>{assert.equal(listeners.get(kind),fn);listeners.delete(kind);},
  requestVideoFrameCallback:fn=>{const id=++counter;callbacks.set(id,fn);return id;},cancelVideoFrameCallback:id=>{callbacks.delete(id);events.push('cancel');}};
 const scope={location:{origin},performance,document:{querySelectorAll:()=>[video],querySelector:()=>({classList:{contains:()=>true}})},
  navigator:{serviceWorker:{controller:{state:'activated',scriptURL:origin+'/service-worker.js?v=55'}}},URL,video};
 const call=operation=>runInNewContext('('+offlineSeekMedia.toString()+')(input)',Object.assign(scope,{input:{origin,operation}}));
 return {scope,video,listeners,callbacks,events,call,frame:mediaTime=>{const [id,fn]=callbacks.entries().next().value;callbacks.delete(id);fn(0,{mediaTime,presentedFrames:mediaTime*10});}};
}
function networkPeer(){
 const listeners=new Map(),attachments=[],media=mediaPeer();
 const page={url:()=>origin+'/offline?job='+jobID,on:(name,fn)=>listeners.set(name,fn),off:(name,fn)=>{assert.equal(listeners.get(name),fn);listeners.delete(name);},
  evaluate:async(fn,input)=>fn===offlineSeekStorage?{available:false,backend:'unavailable',size:null,bytes:null,chunks:null,opfsSize:null}
   :runInNewContext('('+fn.toString()+')(input)',Object.assign(media.scope,{input}))};
 const info={attach:async(name,{body})=>{assert.equal(name,'offline-seek-failure');attachments.push(JSON.parse(body));}};
 return {page,listeners,attachments,media,info};
}
test('untrusted authority and canonical IDs reject with zero listeners, evaluation or writes',()=>{
 for(const baseURL of [undefined,'','foreign','https://foreign.example','http://user@localhost:38127','http://localhost:0',
  origin+'/private',origin+'/?unknown','x'.repeat(2049)]){
  const peer=networkPeer();assert.throws(()=>offlineSeekWitness(peer.page,baseURL,jobID));assert.equal(peer.listeners.size,0);
 }
 for(const id of [undefined,null,'','ABCDEF0123456789','../private','1'.repeat(17)]){
  const peer=networkPeer();assert.throws(()=>offlineSeekWitness(peer.page,origin,id));assert.equal(peer.listeners.size,0);
 }
 const peer=networkPeer();peer.page.url=()=> 'http://127.0.0.1:38128/';assert.throws(()=>offlineSeekWitness(peer.page,origin,jobID));assert.equal(peer.listeners.size,0);
});
test('RVFC first post-action callbacks include pending frames and all listeners cancel',()=>{
 const peer=mediaPeer();assert.equal(peer.call('arm'),true);peer.frame(0.1);
 peer.video.tagName='VIDEO';
 runInNewContext('('+offlineSeekAction.toString()+')(video,origin)',Object.assign(peer.scope,{origin}));
 assert.equal(peer.video.currentTime,4);peer.frame(4);peer.frame(4.1);
 for(let i=2;i<16;i++)peer.frame(4+i/10);
 const value=peer.call('snapshot');assert.equal(value.frames.length,16);assert.equal(value.frames[0].mediaTime,4);
 assert.ok(value.frames.every(frame=>frame.afterSeek&&frame.pending));assert.equal(value.frameLimitReached,true);
 assert.equal(value.workerVersion55,true);
 peer.call('stop');assert.equal(peer.listeners.size,0);assert.equal(peer.callbacks.size,0);
});
test('seek mark and same currentTime=4 setter are atomic and reject foreign origin before either effect',()=>{
 const order=[],scope={location:{origin},__kinosailOfflineSeekWitness:{seek:()=>order.push('mark')}};
 const video={tagName:'VIDEO',get currentTime(){return 0;},set currentTime(value){order.push(value);}};
 runInNewContext('('+offlineSeekAction.toString()+')(video,origin)',{...scope,video,origin});assert.deepEqual(order,['mark',4]);
 order.length=0;
 for(const bad of [undefined,'https://foreign.example','http://127.0.0.1:38128'])
  assert.throws(()=>runInNewContext('('+offlineSeekAction.toString()+')(video,origin)',{...scope,video,origin:bad}));
 assert.deepEqual(order,[]);
 const original=new Error('original setter');Object.defineProperty(video,'currentTime',{get:()=>0,set:()=>{throw original;}});
 assert.throws(()=>runInNewContext('('+offlineSeekAction.toString()+')(video,origin)',{...scope,video,origin}),error=>error===original);
});
function request(url=origin+'/offline-media/1123456789abcdef/'+jobID,range='bytes=2-5'){
 return {url:()=>url,method:()=> 'GET',headers:()=>({range,authorization:'private-secret'}),failure:()=>({errorText:'net::ERR_CONNECTION_RESET private-secret'})};
}
test('owned ranges/status/SW facts are bounded; foreign URLs, queries and secrets never enter attachment',async()=>{
 const peer=networkPeer(),witness=offlineSeekWitness(peer.page,origin,jobID);await witness.arm();witness.disconnected();
 const req=request();peer.listeners.get('request')(req);
 peer.listeners.get('response')({request:()=>req,headers:()=>({'content-range':'bytes 2-5/33','content-length':'4',cookie:'private-secret'}),status:()=>206,fromServiceWorker:()=>true});
 peer.listeners.get('requestfailed')(req);
 for(const url of ['https://foreign.example/offline-media/1123456789abcdef/'+jobID,origin+'/offline-media/1123456789abcdef/'+jobID+'?private',
   origin+'/offline-media/1123456789abcdef/2123456789abcdef','x'.repeat(2049)])peer.listeners.get('request')(request(url));
 await witness.attachFailure(peer.info);const result=peer.attachments[0];
 assert.equal(result.requests.length,3);assert.deepEqual(result.requests[0].range,{start:2,end:5});
 assert.deepEqual(result.requests[1].contentRange,{start:2,end:5,total:33});assert.equal(result.requests[1].fromServiceWorker,true);
 assert.equal(result.requests[2].family,'reset');assert.equal(result.disconnected,true);
 assert.ok(!JSON.stringify(result).includes('private'));assert.ok(!JSON.stringify(result).includes(jobID));assert.ok(!JSON.stringify(result).includes(origin));
 await witness.stop();assert.equal(peer.listeners.size,0);assert.equal(peer.media.listeners.size,0);
});
test('invalid header shapes and oversized rings remain closed numeric observations',async()=>{
 const peer=networkPeer(),witness=offlineSeekWitness(peer.page,origin,jobID);
 for(const range of ['bytes=-2-5','bytes=5-2','bytes=0-99999999999999','x'.repeat(81),'bytes=2-'])
  peer.listeners.get('request')(request(undefined,range));
 for(let i=0;i<25;i++)peer.listeners.get('request')(request());
 await witness.attachFailure(peer.info);const value=peer.attachments[0];assert.equal(value.requests.length,16);assert.equal(value.requestOverflow,true);
 await witness.stop();
});
test('invalid browser snapshots, lost document, and failed attachment preserve the original test error',async()=>{
 const peer=networkPeer(),witness=offlineSeekWitness(peer.page,origin,jobID);peer.page.evaluate=async()=>({unknown:'private-secret'});
 assert.equal(await witness.attachFailure(peer.info),true);assert.equal(peer.attachments[0].media,null);assert.equal(peer.attachments[0].storage,null);
 const original=new Error('original seek assertion');let caught;
 try{throw original;}catch(error){await witness.attachFailure({attach:async()=>{throw new Error('attachment');}});caught=error;}
 assert.equal(caught,original);peer.page.url=()=> 'https://foreign.example/';await witness.stop();assert.equal(peer.listeners.size,0);
});
test('missing OPFS metadata database is unavailable without any open or upgrade',async()=>{
 let opens=0;const result=await runInNewContext('('+offlineSeekStorage.toString()+')(input)',{
  input:{origin,jobID},location:{origin},setTimeout,clearTimeout,indexedDB:{databases:async()=>[],open:()=>{opens++;}}});
 assert.equal(result.available,false);assert.equal(opens,0);
 for(const input of [{origin:'https://foreign.example',jobID},{origin,jobID:'bad'},{origin,jobID,unknown:true}])
  await assert.rejects(runInNewContext('('+offlineSeekStorage.toString()+')(input)',{input,location:{origin},indexedDB:{databases:()=>{throw new Error('must not read');}}}));
});
test('available storage facts reject missing or fractional sizes before attachment',async()=>{
 for(const values of [{size:null},{bytes:null},{chunks:null},{size:1.5},{chunks:1.5}]) {
  const peer=networkPeer(),witness=offlineSeekWitness(peer.page,origin,jobID);
  peer.page.evaluate=async()=>({available:true,backend:'opfs',size:33,bytes:33,chunks:3,opfsSize:null,...values});
  await witness.attachFailure(peer.info);assert.equal(peer.attachments[0].storage,null);await witness.stop();
 }
});
test('CI executes this exact diagnostic control and workflow binds its imported source',()=>{
 const app=readFileSync(new URL('../../.github/workflows/app.yml',import.meta.url),'utf8');
 assert.equal(app.split('\n').filter(line=>line.includes('run: node --test')&&line.includes('scripts/testing/offline-seek-witness.test.mjs')).length,1);
 const workflow=readFileSync(new URL('../../.github/workflows/layout-stability.yml',import.meta.url),'utf8');
 assert.ok(workflow.includes("'apps/player/e2e/offline-seek-witness.mjs'"));
 for(const path of ['apps/player/e2e/offline-seek-witness.mjs','scripts/testing/offline-seek-witness.test.mjs'])
  assert.equal(workflow.split('  workflow_dispatch:',1)[0].split('\n').filter(line=>line.includes('"'+path+'"')).length,1);
});
