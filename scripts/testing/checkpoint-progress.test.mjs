import {test} from 'node:test';
import assert from 'node:assert/strict';
import {checkpointSeconds,prepareSavedPositionBaseline} from '../../apps/player/e2e/checkpoint-progress.ts';

// Failure modes: omitted wire zero becomes NaN; strings/null become numbers;
// invalid or excessive input changes state; independent fixture leaks near-end
// progress; fixture preparation is accidentally applied to durability loops.
test('public omitted-zero and finite numeric seconds keep the exact wire contract',()=>{
 assert.equal(checkpointSeconds({watched:true,revision:2}),0);
 for(const seconds of [0,0.25,29.687579807,31536000,1000000000])assert.equal(checkpointSeconds({seconds}),seconds);
});
test('missing state, malformed/type/range/cardinality/unknown inputs reject without side effects',()=>{
 for(const state of [null,undefined,[],true,{seconds:undefined},{seconds:null},{seconds:'0'},{seconds:'private-marker'},{seconds:NaN},{seconds:Infinity},{seconds:-1},{seconds:1000000001},{unknown:1},Object.fromEntries(Array.from({length:1000},(_,n)=>['field'+n,n]))])assert.throws(()=>checkpointSeconds(state));
});
const watch='/watch/0123456789abcdef';
function pageFor(csrf='synthetic-csrf'){
 const calls=[];let current={seconds:29.687579807,revision:1,session:'prior-case-session'};
 return {calls,url:()=> 'https://owned.fixture/?q=Checkpoint',locator:()=>({getAttribute:async()=>csrf}),request:{
  get:async path=>{calls.push(['GET',path]);return {status:()=>200,json:async()=>({item:{progress:current}})};},
  put:async(path,options)=>{calls.push(['PUT',path,options]);current={revision:1,session:options.data.session};return {status:()=>200};}
 }};
}
test('independent saved-position preparation verifies prior then accepted wire zero through public API',async()=>{
 const page=pageFor();const result=await prepareSavedPositionBaseline(page,watch);
 assert.deepEqual(result,{beforeSeconds:29.687579807,afterSeconds:0});
 assert.deepEqual(page.calls.map(call=>call[0]),['GET','PUT','GET']);
 assert.deepEqual({...page.calls[1][2].data,session:'synthetic'},{seconds:0,watched:false,session:'synthetic',revision:1});
 assert.match(page.calls[1][2].data.session,/^[a-f0-9-]{36}$/);
 assert.equal(page.calls[1][2].headers.Origin,'https://owned.fixture');
});
test('rejected watch and CSRF values cannot issue a public state mutation',async()=>{
 for(const path of ['',null,'/watch/not-an-id','/watch/'+'a'.repeat(10000),'/watch/0123456789abcdef?unknown=1','https://foreign.fixture/watch/0123456789abcdef']){const page=pageFor();await assert.rejects(prepareSavedPositionBaseline(page,path));assert.equal(page.calls.length,0);}
 for(const csrf of [null,'','private\r\nheader','x'.repeat(513)]){const page=pageFor(csrf);await assert.rejects(prepareSavedPositionBaseline(page,watch));assert.equal(page.calls.filter(call=>call[0]==='PUT').length,0);}
});

test("independent preparations use fresh sessions across cases without stale-revision rejection",async()=>{
 const page=pageFor();const seen=new Set(),put=page.request.put;
 page.request.put=async(path,options)=>{if(seen.has(options.data.session))return {status:()=>409};seen.add(options.data.session);return put(path,options);};
 await prepareSavedPositionBaseline(page,watch);await prepareSavedPositionBaseline(page,watch);
 assert.equal(seen.size,2);
});

function reviewPageFor(prior,accepted){let calls=[],read=0;return{calls,url:()=> 'https://owned.fixture/',locator:()=>({getAttribute:async()=> 'synthetic-csrf'}),request:{get:async()=>({status:()=>200,json:async()=>({item:{progress:read++===0?prior:accepted}})}),put:async(_,o)=>{calls.push(o);if(!accepted)accepted={session:o.data.session,revision:1};return{status:()=>200};}}};}
test('falsey malformed prior public states cannot mutate the fixture',async()=>{for(const prior of [null,undefined,false,0,'']){const p=reviewPageFor(prior);await assert.rejects(prepareSavedPositionBaseline(p,watch));assert.equal(p.calls.length,0);}});
test('accepted readback must be the submitted unwatched session and revision',async()=>{for(const accepted of [{session:'foreign',revision:1},{revision:1},{session:'foreign',revision:0},{watched:'true',revision:1},{watched:true,revision:1}]){const p=reviewPageFor({seconds:29.6},accepted);await assert.rejects(prepareSavedPositionBaseline(p,watch));}});

test('oversized or malformed public progress metadata rejects before preparation',async()=>{
 for(const prior of [{session:'x'.repeat(9000)},{dismissed:'false'},{revision:'1'},{readerPage:-1},{readerOffset:Infinity},{updated:[]}]){const p=reviewPageFor(prior);await assert.rejects(prepareSavedPositionBaseline(p,watch));assert.equal(p.calls.length,0);}
});

test('public reader and timestamp model limits reject before PUT',async()=>{
 for(const prior of [{readerOffset:2,readerPage:1},{readerOffset:0.25},{readerPage:10000001},{updated:'not-a-timestamp'},{updated:'2026-02-30T12:00:00Z'},{session:'x'.repeat(129)}]){const p=reviewPageFor(prior);await assert.rejects(prepareSavedPositionBaseline(p,watch));assert.equal(p.calls.length,0);}
});
test('valid public reader/session/time boundaries remain accepted',async()=>{
 for(const prior of [{},{readerOffset:1,readerPage:10000000},{session:'x'.repeat(128)},{updated:'0001-01-01T00:00:00Z'},{updated:'2026-10-07T15:42:10.123456789Z'}]){const p=reviewPageFor(prior);await prepareSavedPositionBaseline(p,watch);assert.equal(p.calls.length,1);}
});
