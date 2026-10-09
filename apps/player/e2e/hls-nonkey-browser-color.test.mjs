// Isolated failure matrix written before metadata/append implementation.
// Existing populated HLS cannot distinguish the direct versus MSE unspecified-color defaults.
// Counterfactuals must never alter source/cache/AVC/media packets or turn original failures green.
// Reject corrupt/extended/zero box extents, duplicate AVC/color entries, unknown tuple, unbound append,
// missing EOF/seek/first frame, dropped pixels or an unsealed public source. Retain all observer rows.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {colorArm, colorFacts, colorFrameQualification} from './hls-nonkey-browser-color.mjs';
import {observeMSEColors} from './hls-nonkey-browser-mse.mjs';
const sha=b=>createHash('sha256').update(b).digest('hex');
const box=(kind,...parts)=>{const body=Buffer.concat(parts),out=Buffer.alloc(body.length+8);
  out.writeUInt32BE(out.length);out.write(kind,4,4,'ascii');body.copy(out,8);return out;};
const nested=avc=>box('moov',box('trak',box('mdia',box('minf',box('stbl',box('stsd',
  Buffer.from([0,0,0,0,0,0,0,1]),avc))))));
const avc=extra=>box('avc1',Buffer.alloc(78),box('avcC',Buffer.from([1,100,0,30,255,225,0])),...extra);
const fixture=extra=>Buffer.concat([box('ftyp',Buffer.from('isom')),nested(avc(extra)),
  box('free',Buffer.from([3,4])),box('moof',Buffer.from([5,6])),box('mdat',Buffer.from([7,8,9]))]);
test('three labeled arms preserve exact original bytes, AVC and every coded fragment',()=>{
  const input=fixture([]),before=Buffer.from(input),base=colorArm(input,'original');
  assert.equal(base.bytes.equals(input),true);assert.equal(base.facts.colr,null);
  for(const [arm,tuple] of [['601',[6,6,6]],['709',[1,1,1]]]){
    const result=colorArm(input,arm);
    assert.deepEqual(colorFacts(result.bytes).colr,{type:'nclx',primaries:tuple[0],transfer:tuple[1],matrix:tuple[2],fullRange:0});
    assert.equal(result.facts.avcSHA256,base.facts.avcSHA256);
    assert.equal(result.facts.fragmentSHA256,base.facts.fragmentSHA256);
    assert.equal(result.facts.inverseOriginalSHA256,sha(input));
    assert.equal(result.bytes.subarray(result.facts.initBytes).equals(base.bytes.subarray(base.facts.initBytes)),true);
    assert.equal(result.bytes.length,input.length+19);
  }
  assert.equal(input.equals(before),true);
});
test('malformed or already specified sample entries never receive a guessed color',()=>{
  const color=box('colr',Buffer.from('nclx'),Buffer.from([0,6,0,6,0,6,0]));
  for(const bytes of [fixture([color]),fixture([box('colr',Buffer.from('nclc'),Buffer.alloc(6))]),
    fixture([box('avcC',Buffer.alloc(7))]),Buffer.alloc(0),Buffer.alloc((8<<20)+1),
    fixture([]).subarray(0,-1),Buffer.from([0,0,0,1,109,111,111,118,0,0,0,0,0,0,0,8]),
    Buffer.from([0,0,0,0,109,111,111,118])])assert.throws(()=>colorArm(bytes,'601'));
  assert.throws(()=>colorArm(fixture([]),'unknown'));
  assert.throws(()=>colorArm(Buffer.concat([fixture([]),nested(avc([]))]),'601'));
  const corrupt=fixture([]);corrupt.writeUInt32BE(0,12);assert.throws(()=>colorArm(corrupt,'601'));
});
test('all byte bindings, exact presented sequence, seek, healthy EOF and actual append are mandatory',()=>{
  const reference=[0,1,2].map(n=>({sha256:String(n).padStart(64,'0')}));
  const phase={rows:reference,ended:true,droppedCallbacks:0,firstCallbackGap:0,captureErrors:[],
    quality:{droppedVideoFrames:0},events:[]};
  const row={result:'observed',referenceComplete:true,publicVideoSuffixQualified:true,
    deliveredBytesVerified:true,metadataByteIdentity:true,appendedBytesVerified:true,
    requestedSource:12.5,forceSeek:{seeking:true,seeked:true,rawTime:12.5},
    observer:{phases:[phase]},adapter:{rawMSE:true},appendFailures:[],endOfStreamReturned:true,
    rawMSEClockFacts:{qualified:true,timestampOffset:12.5,firstClipPTSSeconds:-0.5,firstSourcePTSSeconds:12},
    appendConfiguration:{mode:'segments',appendWindowStart:0,appendWindowEnd:'Infinity',timestampOffset:12.5}};
  assert.equal(colorFrameQualification(row,reference,[0,1,2]).qualified,true);
  for(const bad of [{deliveredBytesVerified:false},{metadataByteIdentity:false},{appendedBytesVerified:false},
    {referenceComplete:false},{publicVideoSuffixQualified:false},{result:'observation-failed'},
    {pageErrors:1},{snapshotFailure:true},{appendFailures:['append']},{adapter:{rawMSE:false}},
    {endOfStreamReturned:false},{rawMSEClockFacts:{qualified:false}},{appendConfiguration:{mode:'sequence'}},
    {requestedSource:NaN},{requestedSource:Infinity},
    {forceSeek:{seeking:false,seeked:true,rawTime:12.5}},{forceSeek:{seeking:true,seeked:true,rawTime:12.5001}}])
    assert.equal(colorFrameQualification({...row,...bad},reference,[0,1,2]).qualified,false);
  for(const bad of [{ended:false},{rows:reference.slice(1)},{droppedCallbacks:1},{firstCallbackGap:1},
    {captureErrors:['capture']},{quality:{droppedVideoFrames:1}},{events:[{name:'error',errorCode:3}]}])
    assert.equal(colorFrameQualification({...row,observer:{phases:[{...phase,...bad}]}},reference,[0,1,2]).qualified,false);
});
test('new component setup errors retain a failure row and preserve original audio evidence',async()=>{
  const audio={label:'original-audio',qualification:{qualified:false}},retained=[audio];
  await observeMSEColors(null,'',{},Buffer.alloc(0),[],true,retained);
  assert.equal(retained[0],audio);assert.equal(retained.length,2);
  assert.equal(retained[1].label,'raw-mse-color-setup');
  assert.equal(retained[1].result,'observation-failed');
  assert.equal(retained[1].failureClass,'color_raw_clock_binding');
});
