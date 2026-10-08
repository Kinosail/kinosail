import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync, readdirSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
let boundary = {};
try {boundary = await import('../fixture-response.mjs');} catch (error) {if (error.code !== 'ERR_MODULE_NOT_FOUND') throw error;}
const source = stripTypeScriptTypes(readFileSync(new URL('./helpers.ts', import.meta.url), 'utf8')).replace(/^import .*;\n/gm, '').replace(/^export \{.*\};\n/gm, '').replace(/^export /gm, '');
const helpers = runInNewContext(`(()=>{${source};return {api,movie};})()`, {Buffer, ...boundary});
const base = 'http://127.0.0.1:39061';
const item = {id:'0123456789abcdef',kind:'video',title:'Example Movie',progress:{}};
const catalog = rows => ({items:rows,view:'movies',sort:'title',query:'',letter:'',letters:null,total:rows.length,offset:0,limit:200});
function peer({raw=JSON.stringify(catalog([item])), origin=base, status=200, responseURL=base+'/api/v1/library?view=movies', type='application/json', declared}={}) {
 const effects = {evaluate:0,fetch:0,downstream:0};
 const browser = {evaluate: async (callback,value) => {
  effects.evaluate++;
  return runInNewContext(`(${callback.toString()})(value)`, {value,location:{origin},document:{querySelector:()=>({content:'owned-csrf'})},
   fetch:async()=>{effects.fetch++;const headers={'Content-Type':type};if(declared!==undefined)headers['Content-Length']=declared;
    const response=new Response(status===204?null:raw,{status,headers});Object.defineProperty(response,'url',{value:responseURL});return response;},
   TextDecoder,Uint8Array,AbortSignal,JSON,URL});
 }};
 return {browser,effects,run:()=>helpers.movie(browser,base),api:(...args)=>helpers.api(browser,args.length?args[0]:base,args[1]??'/api/v1/library?view=movies')};
}

for(const value of [undefined,null,7,'','not-url','x'.repeat(2049),'https://remote.invalid:39061','http://localhost:39061','http://127.0.0.1:0','http://127.0.0.1:80','http://127.0.0.1:65536','http://user:secret@127.0.0.1:39061','http://127.0.0.1:39061?next=/','http://127.0.0.1:39061#x','http://127.0.0.1:39061/path'])
 test('invalid runtime base URL shape rejects without evaluation: '+String(value).slice(0,40),async()=>{
  const p=peer();await assert.rejects(p.api(value));assert.equal(p.effects.evaluate,0);assert.equal(p.effects.fetch,0);
 });

test('different high browser port rejects before HTTP effects',async()=>{const p=peer({origin:'http://127.0.0.1:39062'});await assert.rejects(p.run());assert.equal(p.effects.fetch,0);});
test('real callback returns the unique canonical fixture before any downstream action',async()=>{const p=peer();const found=await p.run();assert.equal(found.id,item.id);assert.equal(found.title,item.title);assert.equal(p.effects.fetch,1);assert.equal(p.effects.downstream,0);});

for(const [name,options] of [
 ['duplicate JSON key',{raw:JSON.stringify(catalog([item])).replace('"id":"0123456789abcdef"','"id":"0123456789abcdef","id":"0123456789abcdef"')}],
 ['escaped duplicate key',{raw:JSON.stringify(catalog([item])).replace('"id":"0123456789abcdef"','"id":"0123456789abcdef","\\u0069d":"0123456789abcdef"')}],
 ['invalid UTF8',{raw:Uint8Array.from([0xc3,0x28])}],['malformed',{raw:'{'}],['trailing',{raw:'{}{}'}],['oversized',{raw:' '.repeat(524289)}],
 ['declared oversized',{declared:'524289'}],['nonfinite exponent',{raw:'{"value":1e400}'}],['unsafe precision',{raw:'{"value":9007199254740993}'}],
 ['depth',{raw:'['.repeat(18)+'0'+']'.repeat(18)}],['array cardinality',{raw:JSON.stringify(Array(1001).fill(0))}],
 ['wrong content type',{type:'text/html'}],['foreign response',{responseURL:'http://127.0.0.1:39062/api/v1/library'}],['HTTP unauthorized',{status:401}],
 ['missing catalog',{raw:'{}'}],['unknown catalog field',{raw:JSON.stringify({...catalog([item]),extra:true})}],
 ['missing fixture',{raw:JSON.stringify(catalog([]))}],['duplicate fixture title',{raw:JSON.stringify(catalog([item,{...item,id:'1111111111111111'}]))}],
 ['duplicate unrelated ID',{raw:JSON.stringify(catalog([item,{...item,title:'Other'}]))}],['unsafe fixture ID',{raw:JSON.stringify(catalog([{...item,id:'../foreign'}]))}],
 ['unknown item field',{raw:JSON.stringify(catalog([{...item,extra:true}]))}],['unknown kind',{raw:JSON.stringify(catalog([{...item,kind:'unknown'}]))}],
 ['oversized title',{raw:JSON.stringify(catalog([{...item,title:'x'.repeat(513)}]))}],['missing progress',{raw:JSON.stringify(catalog([{id:item.id,title:item.title,kind:item.kind}]))}],
 ['invalid progress',{raw:JSON.stringify(catalog([{...item,progress:{seconds:1000000001}}]))}]
]) test('invalid response/target rejects before downstream action: '+name,async()=>{
 const p=peer(options);await assert.rejects(async()=>{const selected=await p.run();p.effects.downstream++;return selected;});assert.equal(p.effects.downstream,0);
});

test('empty 204 response remains an observable successful API status',async()=>{const p=peer({status:204});const result=await p.api();assert.equal(result.status,204);assert.equal(result.data,null);});

for(const [name,raw] of [['nonfinite positive','{"value":1e400}'],['nonfinite negative','{"value":-1e400}'],['unsafe integer','{"value":9007199254740993}'],['depth','['.repeat(18)+'0'+']'.repeat(18)],['cardinality',JSON.stringify(Array(1001).fill(0))]]) test('strict API decoding rejects '+name,async()=>{const p=peer({raw});await assert.rejects(p.api());});

for(const file of readdirSync(new URL('.',import.meta.url)).filter(name=>name.endsWith('.e2e.ts'))) {
 test('registered SDK journeys reject an invalid base URL shape before setup/navigation: '+file,async()=>{
  const callbacks=[];const register=(...args)=>callbacks.push(args.at(-1));register.setup=register;
  const code=stripTypeScriptTypes(readFileSync(new URL(file,import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
  let effects=0;
  runInNewContext(code,{test:register,describe:(...args)=>args.at(-1)(),...boundary,
   fetch:async()=>{effects++;throw new Error('unexpected setup write');},URL,process:{env:{}},
   api:helpers.api,movie:helpers.movie,expect:()=>{throw new Error('unexpected assertion');}});
  assert.ok(callbacks.length>0);
  for(const callback of callbacks) {
   await assert.rejects(callback({app:{baseUrl:'http://127.0.0.1:0',open:async()=>{effects++;throw new Error('unexpected navigation');}},browser:{}}));
   assert.equal(effects,0);
  }
 });
}
