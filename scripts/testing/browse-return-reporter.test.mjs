import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import {createHash} from 'node:crypto';

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
