import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
const origin='http://127.0.0.1:38127',progress='/progress/0123456789abcdef';
async function callback() {
 let journey,handler,predicate;
 const register=(name,...args)=>{if(name==='failed progress save does not stop playback flow')journey=args.at(-1);};
 Object.assign(register,{skip(){},use(){},beforeEach(){}});
 const expect=value=>({toBe:expected=>assert.equal(value,expected)});
 expect.poll=()=>({toBeGreaterThanOrEqual:async()=>{},toBeGreaterThan:async()=>{}});
 const source=stripTypeScriptTypes(readFileSync(new URL('../../apps/player/e2e/playback-startup.spec.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
 runInNewContext(source,{test:register,expect,process:{env:{KINOSAIL_TEST_INSTANCE:'1'}},URL,URLSearchParams,login:async()=>{}});
 const page={getByRole:()=>({click:async()=>{}}),locator:()=>({getAttribute:async()=>progress}),url:()=>origin+'/watch/0123456789abcdef',route:async(match,fn)=>{predicate=match;handler=fn;throw Error('controlled registration stop');}};
 await assert.rejects(journey({page,baseURL:origin}),/controlled registration stop/);
 return {handler,predicate};
}
const valid='seconds=0&session=fixture-session&revision=1&watched=true';
for(const [name,method,body] of [
 ['wrong method','GET',valid],['missing body','POST',''],['duplicate watched','POST',valid+'&watched=false'],['unknown field','POST',valid+'&extra=x'],
 ['oversized','POST','x'.repeat(4097)],['malformed encoding','POST',valid.replace('fixture-session','%FF')],['unknown watched','POST',valid.replace('true','yes')],
 ['nonfinite seconds','POST',valid.replace('seconds=0','seconds=Infinity')],['negative seconds','POST',valid.replace('seconds=0','seconds=-1')],
 ['oversized seconds','POST',valid.replace('seconds=0','seconds=1000000001')],['unsafe revision','POST',valid.replace('revision=1','revision=9007199254740993')],
 ['missing session','POST',valid.replace('session=fixture-session','session=')]
]) test(`actual failed-save callback rejects ${name} before injected abort`,async()=>{
 const {handler}=await callback();let aborted=0;
 await assert.rejects(handler({request:()=>({method:()=>method,postData:()=>body}),abort:async()=>{aborted++;}}));
 assert.equal(aborted,0);
});
for(const watched of ['true','false'])test(`actual callback admits canonical ${watched} and only the owned endpoint`,async()=>{
 const {handler,predicate}=await callback();let aborted=0;
 assert.equal(predicate(new URL(origin+progress)),true);assert.equal(predicate(new URL('http://127.0.0.1:38128'+progress)),false);
 await handler({request:()=>({method:()=> 'POST',postData:()=>valid.replace('true',watched)}),abort:async()=>{aborted++;}});assert.equal(aborted,1);
});
