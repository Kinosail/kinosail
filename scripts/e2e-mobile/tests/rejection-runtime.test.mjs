import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, rmSync, symlinkSync, statSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { createHash } from 'node:crypto';
const script=resolve('rejection.mjs');
function fixture(t){const cwd=mkdtempSync(join(tmpdir(),'kino-rejection-contract-'));t.after(()=>rmSync(cwd,{recursive:true,force:true}));const foreign=join(cwd,'foreign');writeFileSync(foreign,'PRIVATE-UNREGISTERED-SENTINEL');symlinkSync(foreign,join(cwd,'.e2e'));const env={PATH:process.env.PATH,GITHUB_ACTIONS:'true',GITHUB_SHA:'a'.repeat(40),GITHUB_RUN_ID:'12345',GITHUB_RUN_ATTEMPT:'1',GITHUB_JOB:'ios',NATIVE_OUTCOME:'failure',CLEANUP_OUTCOME:'failure',PRIVACY_OUTCOME:'skipped'};return{cwd,foreign,env};}
function run(f,args=[]){return spawnSync(process.execPath,[script,...args],{cwd:f.cwd,env:f.env,encoding:'utf8',timeout:10000});}
for(const outcomes of [{native:'failure',cleanup:'failure',privacy:'skipped'},{native:'failure',cleanup:'success',privacy:'failure'},{native:'cancelled',cleanup:'cancelled',privacy:'skipped'}])test(`metadata-only rejection artifact preserves ${outcomes.native}/${outcomes.cleanup}/${outcomes.privacy} without reading private output`,t=>{
 const f=fixture(t);Object.assign(f.env,{NATIVE_OUTCOME:outcomes.native,CLEANUP_OUTCOME:outcomes.cleanup,PRIVACY_OUTCOME:outcomes.privacy});const result=run(f);assert.equal(result.status,0,result.stderr);
 const path=join(f.cwd,'.e2e-rejection/receipt.json'),bytes=readFileSync(path),receipt=JSON.parse(bytes);assert.equal(receipt.result,1);assert.deepEqual(receipt.outcomes,outcomes);assert.equal(receipt.revision,f.env.GITHUB_SHA);assert.equal(receipt.command,'python3 hosted.py ios');assert.equal(bytes.includes(Buffer.from('PRIVATE-UNREGISTERED-SENTINEL')),false);
 assert.equal(readFileSync(f.foreign,'utf8'),'PRIVATE-UNREGISTERED-SENTINEL');assert.equal(statSync(path).mode&0o777,0o600);
 const hash=readFileSync(join(f.cwd,'.e2e-rejection/SHA256SUMS'),'utf8');assert.equal(hash,createHash('sha256').update(bytes).digest('hex')+'  receipt.json\n');
});
for(const scenario of ['local','sha-missing','sha-invalid','run-oversized','attempt-invalid','job-invalid','outcome-unknown','outcome-missing','privacy-success','extra-arg','output-existing','output-symlink'])test(`rejection metadata rejects ${scenario} without touching raw or foreign paths`,t=>{
 const f=fixture(t);let args=[];
 if(scenario==='local')f.env.GITHUB_ACTIONS='false';if(scenario==='sha-missing')delete f.env.GITHUB_SHA;if(scenario==='sha-invalid')f.env.GITHUB_SHA='PRIVATE-UNREGISTERED-SENTINEL';if(scenario==='run-oversized')f.env.GITHUB_RUN_ID='1'.repeat(21);if(scenario==='attempt-invalid')f.env.GITHUB_RUN_ATTEMPT='-1';if(scenario==='job-invalid')f.env.GITHUB_JOB='tvos';if(scenario==='outcome-unknown')f.env.CLEANUP_OUTCOME='PRIVATE-UNREGISTERED-SENTINEL';if(scenario==='outcome-missing')delete f.env.PRIVACY_OUTCOME;if(scenario==='privacy-success')f.env.PRIVACY_OUTCOME='success';if(scenario==='extra-arg')args=['unexpected'];
 if(scenario==='output-existing'){mkdirSync(join(f.cwd,'.e2e-rejection'));writeFileSync(join(f.cwd,'.e2e-rejection/sentinel'),'PRESERVE');}
 if(scenario==='output-symlink')symlinkSync(f.foreign,join(f.cwd,'.e2e-rejection'));
 const result=run(f,args);assert.notEqual(result.status,0);assert.equal(existsSync(join(f.cwd,'.e2e-rejection/receipt.json')),false);assert.equal(readFileSync(f.foreign,'utf8'),'PRIVATE-UNREGISTERED-SENTINEL');if(scenario==='output-existing')assert.equal(readFileSync(join(f.cwd,'.e2e-rejection/sentinel'),'utf8'),'PRESERVE');
});
