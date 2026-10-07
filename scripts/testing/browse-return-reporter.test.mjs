import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import {createHash} from 'node:crypto';
import {spawnSync} from 'node:child_process';

// Reporter protocol only: no Playwright, browser, Go or fixture processes.
const path = new URL('../../apps/player/e2e/browse-return-proof-reporter.ts', import.meta.url);
function report(project, fullTitle) {
 const records=[];
 const source=stripTypeScriptTypes(readFileSync(path,'utf8'))
  .replace(/^import .*;\n/gm,'').replace('export default class','class')+'\n globalThis.Reporter=BrowseReturnProofReporter;';
 const context={createHash,Buffer,process:{env:{KINOSAIL_BROWSE_RETURN_CASES:'navigation'}},console:{log:value=>records.push(value)}};
 runInNewContext(source,context);
 const reporter=new context.Reporter();
 const title='Home keeps its exact return after Mark watched';
 const item={title,location:{file:'/private/fixture/watch-navigation.spec.ts'},parent:{project:()=>({name:project})},
  titlePath:()=>['',project,'watch-navigation.spec.ts',fullTitle],expectedStatus:'passed'};
 reporter.onBegin({projects:[{name:project}]},{allTests:()=>[item]});
 reporter.onTestEnd(item,{duration:1,retry:0,status:'passed',attachments:[],errors:[]});
 reporter.onEnd({status:'passed'});
 assert.equal(records.length,1);
 return JSON.parse(records[0].slice('Q14_PROOF_RESULT '.length));
}
test('navigation report emits exact file full title and actual project identity',()=>{
 const title='Home keeps its exact return after Mark watched';
 for(const project of ['chromium','firefox','webkit']) {
  const value=report(project,title);
  assert.equal(value.schemaVersion,3);assert.equal(value.project,project);
  assert.deepEqual(value.collected,[{file:'watch-navigation.spec.ts',title,fullTitle:title}]);
  assert.equal(value.cases[0].fullTitle,title);assert.equal(value.errors.length,0);
 }
});
test('unknown project and nested title never become a navigation proof',()=>{
 for(const [project,title] of [['unknown','Home keeps its exact return after Mark watched'],['webkit','unexpected > Home keeps its exact return after Mark watched']]) {
  const value=report(project,title);assert.ok(value.errors.length>0);
  assert.equal(value.collected.length,0);assert.equal(value.cases.length,0);
 }
});

function contextualReport(mode, file, title, parents, collection = false) {
 return reporterOutput(mode,[{file,title,parents}],collection);
}
function reporterOutput(mode, identities, collection) {
 const records=[];
 const source=stripTypeScriptTypes(readFileSync(path,'utf8'))
  .replace(/^import .*;\n/gm,'').replace('export default class','class')+'\n globalThis.Reporter=BrowseReturnProofReporter;';
 const context={createHash,Buffer,process:{env:{KINOSAIL_BROWSE_RETURN_CASES:mode}},console:{log:value=>records.push(value)}};
 runInNewContext(source,context);
 const reporter=new context.Reporter();
 const items=identities.map(({file,title,parents})=>({title,location:{file:'/private/fixture/'+file},parent:{project:()=>({name:'chromium'})},
  titlePath:()=>['','chromium',file,...parents,title],expectedStatus:'passed'}));
 reporter.onBegin({},{allTests:()=>items});
 if(!collection) for(const item of items) reporter.onTestEnd(item,{duration:1,retry:0,status:'passed',attachments:[],errors:[]});
 reporter.onEnd({status:'passed'});
 return JSON.parse(records[0].slice('Q14_PROOF_RESULT '.length));
}
const declaredContexts = [
 ['cold','browse-return-cold.spec.ts','cold native Back restores later Movie cards at 390px','native Back without browser cache'],
 ['cold','browse-return-cold.spec.ts','cold native Back restores later Movie cards at 1440px','native Back without browser cache'],
 ['search','browse-return-cold.spec.ts',"live query uses current URL rather than the document's initial browse key",'live HTMX search then cold playback Back'],
 ['bfcache','browse-return-bfcache.spec.ts','native BFCache preserves loaded Movie DOM without repeated continuation','observed native browser cache'],
];
test('declared nested contexts retain exact collected and completed full identity',()=>{
 for(const [mode,file,title,parent] of declaredContexts) {
  const value=contextualReport(mode,file,title,[parent]);
  assert.equal(value.errors.length,0);
  assert.deepEqual(value.collected,[{file,title,fullTitle:parent+' > '+title}]);
  assert.equal(value.cases[0].fullTitle,parent+' > '+title);
 }
});
test('declared context cannot be omitted, substituted, doubled or moved to another file',()=>{
 for(const [mode,file,title,parent] of declaredContexts) {
  for(const contexts of [[],['unknown'],[parent,'extra'],[parent,parent]]) {
   const value=contextualReport(mode,file,title,contexts);
   assert.ok(value.errors.length>0);assert.equal(value.collected.length,0);assert.equal(value.cases.length,0);
  }
  const value=contextualReport(mode,'browse-return.spec.ts',title,[parent]);
  assert.ok(value.errors.length>0);assert.equal(value.collected.length,0);
 }
});
test('actual nine-identity reporter collection crosses strict Python receipt admission',()=>{
 const identities=[
  ...[390,1440].map(width=>({file:'browse-return.spec.ts',title:'visible Player Back preserves Movies query, offset, extent, focus and scroll at '+width+'px',parents:[]})),
  {file:'browse-return.spec.ts',title:'HTMX title-letter Back fetches current browse data and restores extent without a native pageshow',parents:[]},
  ...['direct Play','details and episode'].map(action=>({file:'browse-return.spec.ts',title:'visible Player Back restores original Shows action via '+action,parents:[]})),
  ...declaredContexts.map(([,file,title,parent])=>({file,title,parents:[parent]})),
 ];
 const value=reporterOutput('all',identities,true);
 assert.equal(value.errors.length,0);assert.equal(value.collected.length,9);
 assert.equal(new Set(value.collected.map(row=>row.file)).size,3);
 const scripts=new URL('../../apps/player/scripts',import.meta.url).pathname;
 const result=spawnSync('python3',['-B','-c','import sys,json;sys.path.insert(0,sys.argv[1]);from campaign_q14_admission import complete;sys.exit(0 if complete(json.load(sys.stdin),True) else 1)',scripts],{input:JSON.stringify(value),encoding:'utf8',timeout:5000,maxBuffer:4096});
 assert.equal(result.error,undefined);assert.equal(result.status,0,result.stderr);
});
