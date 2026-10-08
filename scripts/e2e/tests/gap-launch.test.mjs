import test from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync,symlinkSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import {createHash} from 'node:crypto';
let readGapLaunch;
try {({readGapLaunch}=await import('../gap-launch.mjs'));}catch(error){if(error.code!=='ERR_MODULE_NOT_FOUND')throw error;}
const launcher=new URL('../gap-launch.mjs',import.meta.url).pathname;
const run='20261008T180000Z-12345';
function fixture(){const root=mkdtempSync(join(tmpdir(),'kino-gap-launch-'));mkdirSync(join(root,'bin'));
 mkdirSync(join(root,'.e2e/runs',run),{recursive:true});writeFileSync(join(root,'public-flow-gap.config.ts'),'owned config');
 writeFileSync(join(root,'bin/xvfb-run'),'#!/bin/sh\nprintf "%s\\n" "$@" > argv\nexit 3\n',{mode:0o700});
 return {root,close:()=>rmSync(root,{recursive:true})};}
function execute(f,args){return spawnSync(process.execPath,[launcher,...args],{cwd:f.root,env:{...process.env,PATH:join(f.root,'bin')},encoding:'utf8',timeout:3000});}
test('actual launcher records explicit headed Xvfb command and preserves runner failure',()=>{
 const f=fixture();try{const result=execute(f,[run]);assert.equal(result.status,3,result.stderr);
 const receipt=JSON.parse(readFileSync(join(f.root,'.e2e/runs',run,'gap-launch.json')));
 assert.deepEqual(receipt.command,['xvfb-run','-a','pnpm','exec','e2e','run','--config','public-flow-gap.config.ts','--headed','--target','player','--output','.e2e/runs/'+run+'/gap-runner']);
 assert.equal(receipt.configSHA256,createHash('sha256').update('owned config').digest('hex'));
 const previous=process.cwd();try{process.chdir(f.root);assert.deepEqual(readGapLaunch(run),receipt);}finally{process.chdir(previous);}
 }finally{f.close();}
});
for(const args of [[],['bad'],[run,'extra'],['../foreign'],['x'.repeat(1000)]])test('invalid launcher argv rejects before execution '+args.length,()=>{
 const f=fixture();try{const result=execute(f,args);assert.equal(result.status,2);assert.throws(()=>readFileSync(join(f.root,'argv')));assert.throws(()=>readFileSync(join(f.root,'.e2e/runs',run,'gap-launch.json')));}finally{f.close();}
});
test('launch reader rejects unknown/duplicate/oversized/foreign command and symlink receipt',()=>{
 const f=fixture();const previous=process.cwd();try{assert.equal(execute(f,[run]).status,3);process.chdir(f.root);
 const path=join(f.root,'.e2e/runs',run,'gap-launch.json'),raw=readFileSync(path,'utf8'),good=JSON.parse(raw);
 for(const text of [JSON.stringify({...good,unknown:true}),'{"command":[],"command":[]}', ' '.repeat(4097),JSON.stringify({...good,command:['pnpm','exec','e2e','run'] }),JSON.stringify({...good,configSHA256:'f'.repeat(64)})]){
 writeFileSync(path,text);assert.throws(()=>readGapLaunch(run));}
 rmSync(path);writeFileSync(join(f.root,'foreign'),raw);symlinkSync(join(f.root,'foreign'),path);assert.throws(()=>readGapLaunch(run));
 assert.equal(readFileSync(join(f.root,'foreign'),'utf8'),raw);
 }finally{process.chdir(previous);f.close();}
});
