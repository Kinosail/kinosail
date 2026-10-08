import test from 'node:test';
import assert from 'node:assert/strict';
import {withRecoveryControls} from '../../apps/player/e2e/recovery-controls-owner.mjs';
function fixture(value='off'){const calls=[];return {calls,page:{goto:async path=>calls.push(['goto',path]),getByLabel:()=>({inputValue:async()=>value})},save:async(_,choice)=>calls.push(['save',choice])};}
test('both public control owners are exercised and original setting restored',async()=>{
 for(const initial of ['on','off']){const f=fixture(initial);await withRecoveryControls(f.page,f.save,async mode=>f.calls.push(['use',mode]));assert.deepEqual(f.calls,[['goto','/settings'],['goto','/settings'],['save','on'],['use','custom'],['goto','/settings'],['save','off'],['use','native'],['goto','/settings'],['save',initial]]);}
});
test('unknown missing malformed initial choice rejects before settings mutations or test actions',async()=>{
 for(const value of ['',null,'PRIVATE','on'.repeat(5000)]){const f=fixture(value);await assert.rejects(withRecoveryControls(f.page,f.save,async()=>f.calls.push(['use'])));assert.deepEqual(f.calls,[['goto','/settings']]);}
});
test('failed public save or body still restores initial owner and propagates original error',async()=>{
 for(const stage of ['save','body']){const f=fixture(),error=Error('original');const save=async(page,choice)=>{f.calls.push(['save',choice]);if(stage==='save'&&choice==='on')throw error;};await assert.rejects(withRecoveryControls(f.page,save,async()=>{throw error;}),cause=>cause===error);assert.deepEqual(f.calls.at(-1),['save','off']);}
});
test('restoration failure cannot mask the first recipe error',async()=>{
 const f=fixture(),original=Error('recipe'),cleanup=Error('restore');
 await assert.rejects(withRecoveryControls(f.page,async(_,choice)=>{if(choice==='off')throw cleanup;},async()=>{throw original;}),error=>error===original);
});
