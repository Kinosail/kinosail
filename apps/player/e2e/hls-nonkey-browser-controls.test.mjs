// Failure matrix written before observer implementation:
// A bundled browser may advertise an unavailable H264/AAC decoder; actual decode must run.
// Equal projected source time may skip decoder positioning; unchanged and forced runs stay separate.
// Compositor callback losses, duplicate/unknown pixels, missing EOF or source frames reject qualification.
// RGBA references must come from the same actual browser's full lossless source, bound to CLI frame identity.
// Raw/source coordinates and presentation offsets may differ; no observer row is trimmed or rewritten.
// Complete browser AAC decode is distinct from synchronized HTMLMediaElement audio presentation.
// Authenticated Go planning/preparation/cache/public delivery remain real; unavailable preparation stays unavailable.
// Fixed test-only producer argv cannot redefine readiness/certificates or mutate original source/client bytes.
// Wrong expected sequence, partial PCM, incomplete packet tails, stale media or failed owned joins stay red.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {frameQualification, completeAudioQualification, consumerQualification} from './hls-nonkey-browser-observer.mjs';
const reference = [0,1,2,3].map(n => ({sha256:String(n).padStart(64,'0'), mediaTime:n/24, presentedFrames:n+1}));
const facts = {ended:true, rows:reference, droppedCallbacks:0, captureErrors:[], width:640,height:360};
test('whole requested frame sequence and actual EOF are required', () => {
  assert.equal(frameQualification(facts, reference, [0,1,2,3]).qualified,true);
  assert.equal(frameQualification({...facts,ended:false}, reference, [0,1,2,3]).qualified,false);
  assert.equal(frameQualification({...facts,rows:reference.slice(0,3)}, reference, [0,1,2,3]).qualified,false);
});
test('signed preroll remains a retained failing observation', () => {
  const result=frameQualification(facts,reference,[1,2,3]);
  assert.equal(result.qualified,false); assert.deepEqual(result.actualSourceIndices,[0,1,2,3]);
});
test('missed callbacks, unknown/duplicate pixels and capture failures cannot pass', () => {
  assert.equal(frameQualification({...facts,droppedCallbacks:1},reference,[0,1,2,3]).qualified,false);
  assert.equal(frameQualification({...facts,rows:[...reference,reference[3]]},reference,[0,1,2,3]).qualified,false);
  assert.equal(frameQualification({...facts,captureErrors:['canvas']},reference,[0,1,2,3]).qualified,false);
  assert.equal(frameQualification(facts,[...reference,reference[0]],[0,1,2,3]).qualified,false);
  assert.equal(frameQualification({...facts,rows:[{...reference[0],sha256:'f'.repeat(64)}]},reference,[0]).qualified,false);
});
test('audio requires whole native stereo48k output and exact independent PCM bytes', () => {
  const expected={samples:2,sha256:'a'.repeat(64)};
  const value={samples:2,channels:2,sampleRate:48000,s16leSHA256:expected.sha256,completeDecode:true};
  assert.equal(completeAudioQualification(value,expected).qualified,true);
  for(const bad of [{samples:1},{channels:1},{sampleRate:44100},{s16leSHA256:'b'.repeat(64)},{completeDecode:false}])
    assert.equal(completeAudioQualification({...value,...bad},expected).qualified,false);
});

test('consumer admission requires actual selected HLS with no execution or HTTP errors', () => {
  const phase={...facts,firstCallbackGap:0,quality:{droppedVideoFrames:0,totalVideoFrames:4}};
  const row={result:'observed',pageErrors:0,selectedPublicHLSObserved:true,observer:{phases:[phase]},
    label:'unchanged-client',referenceComplete:true};
  assert.equal(consumerQualification(row,reference,[0,1,2,3]).qualified,true);
  for(const bad of [{result:'observation-failed'},{pageErrors:1},{selectedPublicHLSObserved:false},
    {directMediaRequested:true},{wrongHLSRecipe:true},{httpOverflow:true},{httpBodyFailure:true},
    {publicAssetHashMismatch:true},{snapshotFailure:true}])
    assert.equal(consumerQualification({...row,...bad},reference,[0,1,2,3]).qualified,false);
  for(const bad of [{events:[{name:'error',errorCode:4}]},{firstCallbackGap:1},
    {quality:{droppedVideoFrames:1,totalVideoFrames:4}},{quality:null}])
    assert.equal(consumerQualification({...row,observer:{phases:[{...phase,...bad}]}},reference,[0,1,2,3]).qualified,false);
  const forced={...row,label:'forced-source-coordinate-seek',forceSeek:{seeking:false,seeked:false}};
  assert.equal(consumerQualification(forced,reference,[0,1,2,3]).qualified,false);
  assert.equal(consumerQualification({...forced,forceSeek:{seeking:true,seeked:true}},reference,[0,1,2,3]).qualified,true);
});
