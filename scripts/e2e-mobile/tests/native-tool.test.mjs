import test from 'node:test';
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {resolve} from 'node:path';
test('native version probes preserve strict acceptance and bounded closed failure stages',()=>{
 const result=spawnSync('python3',[resolve(import.meta.dirname,'../../e2e/test_native_tool.py')],{env:{...process.env,PYTHONDONTWRITEBYTECODE:'1'},encoding:'utf8',timeout:15000,maxBuffer:16384});
 assert.equal(result.status,0,result.stderr);
});

import '../../e2e/native-tool-receipt.test.mjs';
