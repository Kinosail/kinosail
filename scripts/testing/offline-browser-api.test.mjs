import test from 'node:test';
import assert from 'node:assert/strict';
import {runInNewContext} from 'node:vm';
import {offlineBrowserAPI as requestOfflineBrowserAPI,offlineFixture,offlineBrowserFetch} from '../../apps/player/e2e/offline-browser-api.mjs';

const origin='http://127.0.0.1:38127';
const offlineBrowserAPI=(page,operation)=>requestOfflineBrowserAPI(page,operation,origin+'/');
const movie={id:'0123456789abcdef',title:'Example Movie',kind:'video',progress:{}};
const track={id:'1123456789abcdef',title:'Example Track One',kind:'audio',progress:{}};
const book={id:'2123456789abcdef',title:'Example Audiobook',kind:'audiobook',progress:{}};
const library=()=>({items:[movie,track,book],view:'all',sort:'title',query:'',letter:'',letters:[],total:3,offset:0,limit:50});
function peer(data=library(),options={}) {
 const effects=[];
 const page={url:()=>options.origin??origin+'/',evaluate:async(fn,input)=>runInNewContext(`(${fn.toString()})(input)`,{
  input,location:{origin},document:{querySelector:()=>({content:options.csrf??'fixture-csrf'})},
  AbortSignal,TextDecoder,Uint8Array,JSON,URL,
  fetch:async(path,init)=>{effects.push({path,init});return {status:options.status??200,url:options.responseURL??origin+path,
   headers:{get:name=>name==='content-type'?(options.type??'application/json'):null},
   body:new Response(options.raw??JSON.stringify(data)).body};}})};
 return {page,effects};
}
test('browser fetch uses real same-origin credentials and the fixed current endpoints',async()=>{
 const {page,effects}=peer();const rows=await offlineBrowserAPI(page,'library');
 assert.equal(offlineFixture(rows,'video').id,movie.id);assert.equal(offlineFixture(rows,'audio').id,track.id);
 assert.equal(offlineFixture(rows,'audiobook').id,book.id);
 assert.equal(effects[0].path,'/api/v1/library');assert.equal(effects[0].init.credentials,'same-origin');
 assert.equal(effects[0].init.method,'GET');
});
test('fixed HA writes keep CSRF and browser origin, and players keep actual identity',async()=>{
 for(const [operation,enabled] of [['home-assistant-on',true],['home-assistant-off',false]]) {
  const {page,effects}=peer({status:'saved'});await offlineBrowserAPI(page,operation);
  assert.equal(effects[0].init.method,'PUT');assert.equal(effects[0].init.headers['X-Kinosail-CSRF'],'fixture-csrf');
  assert.deepEqual(JSON.parse(effects[0].init.body),{enabled});
 }
 const data={players:[{id:'browser-1',name:'Browser',state:'playing',itemId:movie.id,title:movie.title,position:1,duration:12,volume:1,muted:false}]};
 assert.equal((await offlineBrowserAPI(peer(data).page,'players'))[0].itemId,movie.id);
});
test('unknown operation and non-owned origin reject before any evaluation',async()=>{
 let effects=0;const p={url:()=>origin,evaluate:async()=>effects++};
 for(const op of [undefined,null,'unknown','x'.repeat(4097)]) await assert.rejects(offlineBrowserAPI(p,op));
 for(const url of ['https://foreign.example/','http://user@localhost:38127/','ftp://localhost/','http://localhost:0/','x'.repeat(2049)])
  await assert.rejects(offlineBrowserAPI({...p,url:()=>url},'library'));
 assert.equal(effects,0);
});
test('missing or oversized CSRF rejects before settings effects',async()=>{
 for(const csrf of ['', 'x'.repeat(4097)]) {
  const {page,effects}=peer({status:'saved'},{csrf});await assert.rejects(offlineBrowserAPI(page,'home-assistant-on'));
  assert.equal(effects.length,0);
 }
});
test('failed status, foreign final URL and non-JSON reject before downstream fixture actions',async()=>{
 for(const options of [{status:401},{status:303},{responseURL:'https://foreign.example/api/v1/library'},{type:'text/html'},
  {raw:''},{raw:'{'},{raw:'x'.repeat(524289)}]) {
  let actions=0;await assert.rejects(async()=>{const rows=await offlineBrowserAPI(peer({},options).page,'library');offlineFixture(rows,'video');actions++;});
  assert.equal(actions,0);
 }
});
test('malformed catalog envelopes, unknown fields, cardinality and numbers reject',async()=>{
 const bad=[null,[],{error:'authentication required'}, {...library(),private:'unknown'}, {...library(),items:null},
  {...library(),total:-1},{...library(),total:Infinity},{...library(),offset:'0'}, {...library(),items:Array(513).fill(movie)}];
 const missing=library();delete missing.items;bad.push(missing);
 for(const data of bad)await assert.rejects(offlineBrowserAPI(peer(data).page,'library'));
});
test('catalog ID, kind, title, nested progress and duplicate records reject before watch actions',async()=>{
 for(const item of [{...movie,id:'../private'},{...movie,id:'ABCDEF0123456789'},{...movie,title:null},
  {...movie,kind:'unknown'}, {...movie,unknown:true},{...movie,progress:{unknown:true}},
  {...movie,progress:{seconds:-1}},{...movie,cast:[{name:'Actor',unknown:true}]}])
  await assert.rejects(offlineBrowserAPI(peer({...library(),items:[item],total:1}).page,'library'));
 await assert.rejects(offlineBrowserAPI(peer({...library(),items:[movie,movie],total:2}).page,'library'));
 const rows=await offlineBrowserAPI(peer({...library(),items:[movie,{...movie,id:'3123456789abcdef'}],total:2}).page,'library');
 assert.throws(()=>offlineFixture(rows,'video'));
 for(const rows of [[],[{...movie,title:'Other'}],[{...movie,id:'bad'}]])assert.throws(()=>offlineFixture(rows,'video'));
 assert.throws(()=>offlineFixture([movie],'unknown'));
});
test('HA response schema rejects unknown fields and invalid identities/numbers',async()=>{
 for(const data of [{status:'other'},{status:'saved',unknown:true}])await assert.rejects(offlineBrowserAPI(peer(data).page,'home-assistant-on'));
 for(const data of [{players:null},{players:[],unknown:true},{players:[{id:'bad/ID'}]},
  {players:[{id:'browser-1',name:'Browser',state:'unknown',position:0,duration:12,volume:1,muted:false}]}])
  await assert.rejects(offlineBrowserAPI(peer(data).page,'players'));
});

test('duplicate JSON keys including escaped aliases reject before fixture actions',async()=>{
 for(const raw of ['{"status":"other","status":"saved"}', '{"status":"saved","\\u0073tatus":"saved"}'])
  await assert.rejects(offlineBrowserAPI(peer({}, {raw}).page,'home-assistant-on'));
});
test('pinned cookie filter distinguishes HTTP127 relay from localhost and HTTPS',async()=>{
 const {createRequire}=await import('node:module');const {dirname,join}=await import('node:path');
 const {readFileSync}=await import('node:fs');
 let require=createRequire(new URL('../../apps/player/e2e/package.json',import.meta.url));
 require=createRequire(require.resolve('@playwright/test'));require=createRequire(require.resolve('playwright/package.json'));
 const root=dirname(require.resolve('playwright-core/package.json'));
 assert.equal(JSON.parse(readFileSync(join(root,'package.json'))).version,'1.63.0');
 const source=readFileSync(join(root,'lib/coreBundle.js'),'utf8');
 const start=source.indexOf('function filterCookies(cookies, urls) {');
 const end=source.indexOf('function isForbiddenHeader(',start);assert.ok(start>=0&&end>start);
 const filter=runInNewContext(source.slice(start,end)+';filterCookies',{URL});
 const cookie={name:'fixture',value:'synthetic',domain:'127.0.0.1',path:'/',secure:true};
 assert.equal(filter([cookie],['http://127.0.0.1:38127/']).length,0);
 assert.equal(filter([cookie],['https://127.0.0.1:38127/']).length,1);
 assert.equal(filter([{...cookie,domain:'localhost'}],['http://localhost:38127/']).length,1);
});

test('canonical letter objects and nullable letter slices remain admitted',async()=>{
 for(const letters of [null,[{label:'E',count:3,offset:0}]])
  assert.equal((await offlineBrowserAPI(peer({...library(),letters}).page,'library')).length,3);
});
test('wrong-type players and malformed sibling fixture rows reject',async()=>{
 const player={id:123,name:'Browser',state:'playing',position:0,duration:12,volume:1,muted:false};
 await assert.rejects(offlineBrowserAPI(peer({players:[player]}).page,'players'));
 assert.throws(()=>offlineFixture([movie,{unknown:true}],'video'));
});

test('browser callback rejects malformed input and unknown fields before fetch',async()=>{
 for(const input of [null,[],{}, {operation:'library',origin,extra:true}]) {
  let effects=0;
  await assert.rejects(async()=>runInNewContext(`(${offlineBrowserFetch.toString()})(input)`,{
   input,location:{origin},fetch:()=>effects++,AbortSignal}));
  assert.equal(effects,0);
 }
});
test('malformed JSON errors never include private response strings',async()=>{
 await assert.rejects(offlineBrowserAPI(peer({}, {raw:'{"PRIVATE\\x":"secret"}'}).page,'library'),error=>!error.message.includes('PRIVATE')&&!error.message.includes('secret'));
});

test('partial filtered or unknown catalog scope cannot establish unique fixture identity',async()=>{
 for(const patch of [{total:4},{offset:1},{view:'movies'},{sort:'unknown'},{query:'filtered'},{letter:'E'},{limit:201}])
  await assert.rejects(offlineBrowserAPI(peer({...library(),...patch}).page,'library'));
});

test('explicit fixture authority rejects invalid inputs and other loopback processes before evaluation',async()=>{
 let evaluations=0,writes=0,attachments=0;
 const page={url:()=>origin+'/',evaluate:async()=>{evaluations++;writes++;},attach:()=>attachments++};
 for(const expected of [undefined,null,{},'', 'x'.repeat(2049),'not-a-url','ftp://localhost:38127/',
  'https://foreign.example/','http://user:secret@localhost:38127/','http://localhost:0/',
  origin+'/other',origin+'/?x=1',origin+'/#hash','http://127.0.0.1:38128/','http://127.0.0.1:038127/'])
  await assert.rejects(requestOfflineBrowserAPI(page,'home-assistant-on',expected));
 for(const current of ['http://127.0.0.1:38128/','https://127.0.0.1:38127/','http://localhost:38127/',
  'https://foreign.example/','http://user@127.0.0.1:38127/',origin+'/#hash'])
  await assert.rejects(requestOfflineBrowserAPI({...page,url:()=>current},'home-assistant-on',origin+'/'));
 assert.equal(evaluations,0);assert.equal(writes,0);assert.equal(attachments,0);
});
test('the same fixture authority reaches callback and origin change rejects before fetch',async()=>{
 const {page,effects}=peer();let received;
 const evaluate=page.evaluate;page.evaluate=async(fn,input)=>{received=input.origin;return evaluate(fn,input);};
 await requestOfflineBrowserAPI(page,'library',origin+'/');assert.equal(received,origin);assert.equal(effects.length,1);
 let fetches=0;
 await assert.rejects(async()=>runInNewContext(`(${offlineBrowserFetch.toString()})(input)`,{
  input:{operation:'home-assistant-on',origin},location:{origin:'http://127.0.0.1:38128'},
  document:{querySelector:()=>({content:'fixture-csrf'})},fetch:()=>fetches++,AbortSignal}));
 assert.equal(fetches,0);
});
test('fixture selection rejects unknown sibling kinds and duplicate IDs before actions',()=>{
 const row={id:movie.id,kind:movie.kind,title:movie.title};let actions=0;
 for(const rows of [[row,{id:track.id,kind:'unknown',title:'Other'}],
  [row,{id:movie.id,kind:'audio',title:'Other'}]])
  assert.throws(()=>{offlineFixture(rows,'video');actions++;});
 assert.equal(actions,0);
});
