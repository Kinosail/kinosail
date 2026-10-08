import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';

test('actual SDK input collector binds the Owner setup validator import', () => {
 const cwd=fileURLToPath(new URL('..',import.meta.url));
 const script=readFileSync(new URL('../run.sh',import.meta.url),'utf8');
 const line=script.split('\n').find(line=>line.startsWith('shasum -a 256 '));
 assert.ok(line);const command=line.split(' > ')[0];
 assert.match(command,/^shasum -a 256 [A-Za-z0-9.* /_-]+$/);
 const manifest=execFileSync('/bin/bash',['-c',command],{cwd,encoding:'utf8',timeout:10000,maxBuffer:131072});
 const owner=readFileSync(new URL('owner.setup.e2e.ts',import.meta.url),'utf8');
 assert.match(owner,/from '\.\.\/fixture-setup\.mjs'/);
 const bytes=readFileSync(new URL('../fixture-setup.mjs',import.meta.url));
 const hash=createHash('sha256').update(bytes).digest('hex');
 assert.ok(manifest.split('\n').includes(hash+'  fixture-setup.mjs'),'actual collector omitted imported setup validator');
});
