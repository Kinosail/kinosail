// Written-first PCM correspondence controls. No edit/trim/clock or decoder EOF claim.
// Compare every returned public sample at a predeclared offset window; preserve ambiguities.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {installAudioTailMatcher} from './hls-nonkey-browser-audio-tail.mjs';
const saved=globalThis.window;globalThis.window={};installAudioTailMatcher();
const match=window.nonkeyAudioTailMatch;globalThis.window=saved;
const source=()=>[0,1].map(c=>Float32Array.from({length:512},(_,n)=>(n*3+c+1)/2048));
const tail=(channels,first=136)=>channels.map(v=>v.slice(first));
test('whole untouched stereo tail has one exact offset and includes returned EOF',()=>{
  const full=source(),publicPCM=tail(full),result=match(full,publicPCM,128);
  assert.equal(result.qualified,true);assert.deepEqual(result.exactOffsets,[136]);
  assert.equal(result.nominalOffset,128);assert.equal(result.candidateRadius,64);
  assert.equal(result.sourceSamples,512);assert.equal(result.publicSamples,376);
  assert.equal(result.wholeReturnedEOFTail,true);assert.equal(result.candidates.length,129);
  assert.equal(result.candidates.find(v=>v.offset===136).floatBitMismatches,0);
  assert.equal(result.candidates.find(v=>v.offset===136).s16Mismatches,0);
});
test('omission, reorder, channel swap and a single interior changed sample remain failing',()=>{
  const full=source(),correct=tail(full),omit=correct.map(v=>v.slice(1));
  const reorder=correct.map(v=>Float32Array.from(v));[reorder[0][50],reorder[0][51]]=[reorder[0][51],reorder[0][50]];
  const interior=correct.map(v=>Float32Array.from(v));interior[1][173]+=1/65536;
  const missing=correct.map(v=>v.slice(0,-1));
  for(const bad of [omit,reorder,[correct[1],correct[0]],interior,missing])
    assert.equal(match(full,bad,128).qualified,false);
  const changed=match(full,interior,128).candidates.find(v=>v.offset===136);
  assert.equal(changed.floatBitMismatches,1);assert.equal(changed.s16Mismatches,1);
  assert.deepEqual(changed.firstFloatMismatches,[{sample:173,channel:1}]);
});
test('full comparison records interior defects even when the first sample mismatches',()=>{
  const full=source(),bad=tail(full);bad[0][0]+=0.01;bad[1][375]+=0.01;
  const result=match(full,bad,128).candidates.find(v=>v.offset===136);
  assert.equal(result.floatBitMismatches,2);assert.equal(result.s16Mismatches,2);
  assert.deepEqual(result.firstFloatMismatches,[{sample:0,channel:0},{sample:375,channel:1}]);
});
test('quantized equality cannot hide signed-zero float bits or authorize a wrong offset',()=>{
  const full=source();full[0][200]=0;const bad=tail(full);bad[0][64]=-0;
  const result=match(full,bad,128),row=result.candidates.find(v=>v.offset===136);
  assert.equal(row.s16Mismatches,0);assert.equal(row.floatBitMismatches,1);assert.equal(result.qualified,false);
  assert.equal(match(source(),tail(source()),256).qualified,false);
});
test('repeated ambiguous audio and a matched prefix without returned EOF never qualify',()=>{
  const repeated=[new Float32Array(512),new Float32Array(512)];
  const ambiguous=match(repeated,[new Float32Array(376),new Float32Array(376)],128);
  assert.equal(ambiguous.qualified,false);assert.equal(ambiguous.exactOffsets.length>1,true);
  const full=source(),prefix=full.map(v=>v.slice(136,400)),result=match(full,prefix,128);
  assert.deepEqual(result.exactOffsets,[136]);assert.equal(result.wholeReturnedEOFTail,false);
  assert.equal(result.qualified,false);
});
test('nonfinite, unequal stereo extents, invalid rates of search and empty PCM fail closed',()=>{
  const full=source(),bad=tail(full);bad[1][20]=NaN;
  for(const args of [[full,bad,128],[full,[bad[0]],128],[full,[bad[0],bad[1].slice(1)],128],
    [full,[new Float32Array(),new Float32Array()],128],[full,tail(full),NaN],
    [full,tail(full),-1],[full,tail(full),128.5],[full,tail(full),1600001]])
    assert.throws(()=>match(...args),/audio_tail_shape/);
});
