import test from 'node:test';
import assert from 'node:assert/strict';
import {attachPlaybackState} from '../../apps/player/e2e/playback-state-witness.mjs';
const make=(raw,status=200)=>{const rows=[],calls=[];return {rows,calls,page:{request:{get:async(path,options)=>{calls.push([path,options]);return {status:()=>status,text:async()=>raw};}}},info:{attach:async(name,data)=>rows.push(JSON.parse(data.body))}};};
test('backend progress projection contains only bounded facts and exact stage',async()=>{
 const fixture=make(JSON.stringify({item:{title:'PRIVATE',progress:{seconds:12,watched:true}}}));
 await attachPlaybackState(fixture.page,fixture.info,'/watch/example-movie','before-method');
 assert.deepEqual(fixture.rows,[{schemaVersion:1,stage:'before-method',status:200,progress:{seconds:12,watched:true}}]);
 assert.equal(fixture.calls[0][0],'/api/v1/items/example-movie');
});
test('invalid watch or stage has no HTTP or attachment effects',async()=>{
 for(const watch of [null,'/watch/','/watch/'+ 'x'.repeat(129),'/watch/../secret','https://private/watch/item','/watch/id?secret=1']){
 const f=make('{}');await assert.rejects(attachPlaybackState(f.page,f.info,watch,'before-method'));assert.deepEqual(f.calls,[]);assert.deepEqual(f.rows,[]);
 }
 const f=make('{}');await assert.rejects(attachPlaybackState(f.page,f.info,'/watch/item','PRIVATE'));assert.deepEqual(f.calls,[]);
});
test('malformed missing oversized and private backend values publish only a closed unavailable result',async()=>{
 for(const raw of ['bad','x'.repeat(65537),'{}','['.repeat(33)+'0'+']'.repeat(33),'{"item":{"progress":{"seconds":0,"watched":false,"watched":true}}}',JSON.stringify({item:{progress:{seconds:-1,watched:true}}}),JSON.stringify({item:{progress:{seconds:'PRIVATE',watched:'PRIVATE'}}}),JSON.stringify({item:{progress:{seconds:Infinity,watched:false}}})]){
 const f=make(raw);await attachPlaybackState(f.page,f.info,'/watch/item','before-loading');
 assert.deepEqual(f.rows,[{schemaVersion:1,stage:'before-loading',status:200,unavailable:true}]);
 }
});
test('observation and attachment failures do not replace recipe errors',async()=>{
 const f=make('{}');f.page.request.get=async()=>{throw Error('PRIVATE');};
 await attachPlaybackState(f.page,f.info,'/watch/item','before-method');assert.equal(JSON.stringify(f.rows).includes('PRIVATE'),false);
 f.info.attach=async()=>{throw Error('PRIVATE');};await attachPlaybackState(f.page,f.info,'/watch/item','before-method');
});
