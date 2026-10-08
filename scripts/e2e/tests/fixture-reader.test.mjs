import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import * as boundary from '../fixture-response.mjs';
const id='0123456789abcdef';
const book={id,kind:'book',title:'E2E EPUB',progress:{}};
const pages=[{title:'Chapter 1',url:'/read/'+id+'/asset/OEBPS/one.xhtml',number:1},
 {title:'Chapter 2',url:'/read/'+id+'/asset/OEBPS/two.xhtml',number:2}];
const reader={id,title:book.title,type:'epub',pages};
const pdf={...book,title:'E2E PDF'};
const pdfReader={id,title:pdf.title,type:'pdf',pages:[{title:'Document',url:'/read/'+id+'/file',number:1}]};
const copy=value=>structuredClone(value);
function registered(data,status=200) {
 const callbacks=[];const register=(name,callback)=>callbacks.push(callback);register.skip=()=>{};
 const source=stripTypeScriptTypes(readFileSync(new URL('./audio-readers.e2e.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
 const effects={opens:0,assets:0};
 runInNewContext(source,{test:register,describe:(name,options,callback)=>callback(),...boundary,
  expect:value=>({toBe(expected){assert.equal(value,expected);},toBeDefined(){assert.notEqual(value,undefined);},toHaveLength(expected){assert.equal(value.length,expected);}}),
  api:async(browser,base,path)=>path.includes('/reader')?{status,data}:{status:200,data:{items:[book],view:'books',sort:'title',query:'',letter:'',letters:null,total:1,offset:0,limit:200}}});
 return {effects,run:()=>callbacks[1]({app:{baseUrl:'http://127.0.0.1:39061',open:async path=>{if(path!='/settings'){effects.opens++;throw new Error('downstream navigation');}}},
  browser:{title:async()=> 'Kinosail Player',evaluate:async()=>{effects.assets++;}},screen:{}})};
}
const invalid=[['missing',{}],['unknown',{...reader,extra:true}],['wrong ID',{...reader,id:'1111111111111111'}],
 ['unsafe ID',{...reader,id:'../foreign'}],['wrong title',{...reader,title:'Other'}],['wrong type',{...reader,type:'pdf'}],
 ['missing pages',{...reader,pages:null}],['too many',{...reader,pages:[...pages,pages[0]]}],['empty',{...reader,pages:[]}],
 ['foreign URL',{...reader,pages:[{...pages[0],url:'https://foreign.invalid'},pages[1]]}],
 ['query URL',{...reader,pages:[{...pages[0],url:pages[0].url+'?x=1'},pages[1]]}],
 ['wrong page',{...reader,pages:[{...pages[0],number:2},pages[1]]}],
 ['unknown page',{...reader,pages:[{...pages[0],extra:true},pages[1]]}],
 ['wrong chapter title',{...reader,pages:[{...pages[0],title:'Other'},pages[1]]}],
 ['oversized URL',{...reader,pages:[{...pages[0],url:'x'.repeat(4097)},pages[1]]}]];
for(const [name,data] of invalid) test('registered EPUB rejects '+name+' before item navigation or asset fetch',async()=>{
 const p=registered(copy(data));await assert.rejects(p.run());assert.equal(p.effects.opens,0);assert.equal(p.effects.assets,0);
});
test('registered EPUB rejects non-success before item navigation',async()=>{const p=registered(reader,401);await assert.rejects(p.run());assert.equal(p.effects.opens,0);});
test('owned EPUB and PDF reader descriptors retain exact page URLs',()=>{
 assert.deepEqual(boundary.fixtureReader(reader,book),reader);assert.deepEqual(boundary.fixtureReader(pdfReader,pdf),pdfReader);
});
test('reader rejects invalid selected target before accepting response',()=>{
 for(const target of [null,{...book,id:'bad'},{...book,kind:'video'},{...book,title:'Other'},{...book,unknown:true}]) assert.throws(()=>boundary.fixtureReader(reader,target));
});
test('PDF descriptor cannot accept a chapter or foreign file',()=>{
 for(const data of [{...pdfReader,pages:pages},{...pdfReader,pages:[{...pdfReader.pages[0],url:'/read/1111111111111111/file'}]}]) assert.throws(()=>boundary.fixtureReader(data,pdf));
});
test('watch progress retains finite seconds and unknown duration',()=>{
 for(const data of [{seconds:3,duration:24},{seconds:3,duration:0},{seconds:0,duration:0},{seconds:315360000,duration:315360000}]) assert.deepEqual(boundary.fixtureWatchProgress(data),data);
});
for(const data of [null,{},[],{seconds:3},{seconds:3,duration:24,unknown:true},{seconds:-1,duration:24},{seconds:25,duration:24},
 {seconds:Infinity,duration:24},{seconds:NaN,duration:24},{seconds:'3',duration:24},{seconds:3,duration:-1},
 {seconds:315360001,duration:0},{seconds:3,duration:315360001}]) test('invalid watch progress rejects before downstream action '+JSON.stringify(data),()=>{
 let downstream=0;assert.throws(()=>{boundary.fixtureWatchProgress(data);downstream++;});assert.equal(downstream,0);
});
