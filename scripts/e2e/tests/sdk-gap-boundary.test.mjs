import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,mkdtempSync,mkdirSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import * as boundary from '../fixture-response.mjs';
test('all three registered gap journeys reject foreign base URL before any action',async()=>{
 const callbacks=[];const register=(...args)=>callbacks.push(args.at(-1));
 const source=stripTypeScriptTypes(readFileSync(new URL('../deep-tests/public-flow-gap.e2e.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
 let effects=0;
 runInNewContext(source,{test:register,describe:(...args)=>args.at(-1)(),...boundary,process:{env:{}},
  surfaceOf:()=>{effects++;throw new Error('unexpected engine effect');},readGapLaunch:()=>{effects++;},api:()=>{effects++;},movie:()=>{effects++;}});
 assert.equal(callbacks.length,3);
 for(const callback of callbacks){await assert.rejects(callback({app:{baseUrl:'http://127.0.0.1:0',open:()=>{effects++;}},browser:{}}));assert.equal(effects,0);}
});

test('actual deep config passes pinned SDK validator using the existing owned supervisor',async()=>{
 const sdk=new URL(import.meta.resolve('e2e'));
 const {resolveConfig}=await import(new URL('./config/resolve.js',sdk));
 const root=mkdtempSync(join(tmpdir(),'kino-gap-config-')),previous=process.cwd(),old=process.env.KINOSAIL_E2E_GAP_RUN;
 const run='20261008T180000Z-12345';
 try{
  mkdirSync(join(root,'.e2e/runs',run),{recursive:true});mkdirSync(join(root,'bin'));
  writeFileSync(join(root,'public-flow-gap.config.ts'),readFileSync(new URL('../public-flow-gap.config.ts',import.meta.url)));
  writeFileSync(join(root,'bin/xvfb-run'),'#!/bin/sh\nexit 0\n',{mode:0o700});
  const command=spawnSync(process.execPath,[new URL('../gap-launch.mjs',import.meta.url).pathname,run],{cwd:root,env:{...process.env,PATH:join(root,'bin')},encoding:'utf8',timeout:3000});
  assert.equal(command.status,0,command.stderr);process.chdir(root);process.env.KINOSAIL_E2E_GAP_RUN=run;
  const raw=(await import(new URL('../public-flow-gap.config.ts',import.meta.url).href+'?owned='+Date.now())).default;
  const resolved=resolveConfig(raw,{projectRoot:root,env:{},cli:{headed:true}});
  assert.equal(resolved.targets.length,1);assert.equal(resolved.targets[0].name,'player');
  assert.deepEqual(raw.targets[0].app.command.args,['fixture.mjs','player','{port}']);
  assert.deepEqual(raw.tests,['tests/owner.setup.e2e.ts','deep-tests/public-flow-gap.e2e.ts']);
 }finally{process.chdir(previous);if(old===undefined)delete process.env.KINOSAIL_E2E_GAP_RUN;else process.env.KINOSAIL_E2E_GAP_RUN=old;rmSync(root,{recursive:true});}
});
