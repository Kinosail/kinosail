import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync,mkdtempSync,mkdirSync,writeFileSync,rmSync} from 'node:fs';
import {resolve,join} from 'node:path';
import {tmpdir} from 'node:os';
import {sanitize} from '../privacy.mjs';
const sdk=new URL(import.meta.resolve('e2e'));
const {E2EError,serializeError}=await import(new URL('./internal/errors.js',sdk));
const phone=readFileSync(resolve(import.meta.dirname,'phone.e2e.ts'),'utf8');
const helper=resolve(import.meta.dirname,'../install.mjs');
const install=existsSync(helper)?(await import(helper)).installPhone:async device=>{
 assert.ok(phone.includes('await device.installApp(undefined, { reinstall: true });'));
 await device.installApp(undefined,{reinstall:true});
};
function failure(details={stdout:'',stderr:'PRIVATE-SENTINEL uninstall refused',exitCode:1}){
 const cause=Object.assign(new Error('PRIVATE-SENTINEL'),{code:'COMMAND_FAILED',details});
 return Object.assign(new Error('install failed',{cause}),{code:'ENGINE_FAILURE'});
}
async function invoke(error){
 const calls=[];const device={installApp:async(...args)=>{calls.push(args);throw error;}};
 await assert.rejects(install(device),thrown=>thrown===error);
 assert.deepEqual(calls,[[undefined,{reinstall:true}]]);
 assert.equal(error.cause?.message,'PRIVATE-SENTINEL');return error.details?.observed;
}
test('actual phone install keeps original failure and gives only closed command evidence',async()=>{
 const error=failure();const observed=await invoke(error);assert.equal(typeof observed,'string');assert.ok(Buffer.byteLength(observed)<=256);
 assert.equal(observed.includes('PRIVATE-SENTINEL'),false);
 assert.deepEqual(JSON.parse(observed),{stage:'install_app',category:'command_failed',exitCode:1,stdoutBytes:0,stderrBytes:34,packageAbsentText:false});
 const absent=failure({stdout:'Unknown Package com.private.app',stderr:'',exitCode:1});assert.equal(JSON.parse(await invoke(absent)).packageAbsentText,true);
});
test('invalid details never copy private content or infer success',async()=>{
 for(const details of [null,{}, {stdout:'',stderr:'',exitCode:0,extra:'PRIVATE-SENTINEL'},{stdout:[],stderr:'',exitCode:1},{stdout:'',stderr:'',exitCode:0},{stdout:'',stderr:'',exitCode:-129},{stdout:'\ud800',stderr:'',exitCode:1},{stdout:'',stderr:'',exitCode:Infinity},{stdout:'',stderr:'',exitCode:256},{stdout:'x'.repeat(65537),stderr:'',exitCode:1},{stdout:'',stderr:'x'.repeat(8193),exitCode:1}]){
  const error=failure(details);const observed=await invoke(error);assert.equal(JSON.parse(observed).category,'unqualified');assert.equal(observed.includes('PRIVATE-SENTINEL'),false);assert.ok(Buffer.byteLength(observed)<=256);
 }
});
test('cause cycles and opaque errors stay unknown and original assignment failure wins',async()=>{
 const error=failure();error.cause.cause=error;delete error.cause.code;
 const observed=await invoke(error);assert.equal(JSON.parse(observed).category,'unqualified');
 const deep=failure();delete deep.cause.code;
 let cursor=deep.cause;for(let i=0;i<9;i++){cursor.cause=new Error('PRIVATE-SENTINEL');cursor=cursor.cause;}
 cursor.code='COMMAND_FAILED';cursor.details={stdout:'',stderr:'',exitCode:1};
 assert.equal(JSON.parse(await invoke(deep)).category,'unqualified');
 const opaque=failure();Object.defineProperty(opaque.cause,'details',{get(){throw new Error('PRIVATE-SENTINEL')}});
 assert.equal(JSON.parse(await invoke(opaque)).category,'unqualified');
 const frozen=failure();Object.freeze(frozen);await invoke(frozen);assert.equal(frozen.details,undefined);
});
test('successful install invokes reinstall exactly once and does not fabricate a failure',async()=>{
 const calls=[];await install({installApp:async(...args)=>calls.push(args)});assert.deepEqual(calls,[[undefined,{reinstall:true}]]);
});
test('phone callsite retains public adapter and all later media assertions',()=>{
 assert.ok(phone.includes('reinstall: true')||phone.includes('await installPhone(device)'));
 for(const oracle of ['await app.open();','decodedMotion(first, second)','expect(progress.seconds).toBe(saved);','connectionRestored: true'])assert.ok(phone.includes(oracle));
});

test('pinned SDK serializes closed observation after cleanup-safe text sanitization',async t=>{
 const cause=failure().cause,error=new E2EError('infrastructure','ENGINE_FAILURE','install failed',{cause});
 await invoke(error);const serialized=serializeError(error);
 assert.equal(serialized.code,'ENGINE_FAILURE');assert.equal(serialized.category,'infrastructure');assert.equal(serialized.details.observed,error.details.observed);
 const root=mkdtempSync(join(tmpdir(),'kino-install-report-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 mkdirSync(join(root,'sdk'),{mode:0o700});
 writeFileSync(join(root,'sdk/report.json'),JSON.stringify({error:serialized}),{mode:0o600});
 writeFileSync(join(root,'secrets.json'),JSON.stringify({secrets:['PRIVATE-SENTINEL'],stage:'started'}),{mode:0o600});
 sanitize(root,{result:1});const safe=readFileSync(join(root,'published/report.json'),'utf8');
 assert.equal(safe.includes('PRIVATE-SENTINEL'),false);assert.equal(JSON.parse(safe).error.details.observed,error.details.observed);
});
