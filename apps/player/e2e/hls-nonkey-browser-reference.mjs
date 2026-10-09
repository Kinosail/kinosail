import {createHash} from 'node:crypto';
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
export function directReferenceBodyFacts(status,range,bytes,reference){
  if(!bytes.length || bytes.length>2<<20 || !reference.length || reference.length>2<<20)throw Error('reference_body_bound');
  let first=0,last=reference.length-1;
  if(status===206){
    const match=/^bytes ([0-9]{1,10})-([0-9]{1,10})\/([0-9]{1,10})$/.exec(range||'');
    if(!match)throw Error('reference_range_shape');
    first=Number(match[1]);last=Number(match[2]);
    if(Number(match[3])!==reference.length || first>last || last>=reference.length ||
      bytes.length!==last-first+1)throw Error('reference_range_extent');
  }else if(status!==200 || range || bytes.length!==reference.length)throw Error('reference_complete_extent');
  const expected=sha(reference.subarray(first,last+1)),actual=sha(bytes);
  return {status,bytes:bytes.length,first,last,totalBytes:reference.length,
    sha256:actual,expectedSHA256:expected,matched:actual===expected};
}
export function safeBrowserProjection(row){
  const phase=row.observer?.phases?.at(-1),rows=phase?.rows||[];
  const frame=row.frameQualification||row.directQualification;
  const edge=v=>v && ({sha256:v.sha256,width:v.width,height:v.height,mediaTime:v.mediaTime,
    rawTime:v.rawTime,projectedTime:v.projectedTime,presentedFrames:v.presentedFrames});
  return {request:row.request,label:row.label,result:row.result,currentStage:row.currentStage,failureClass:row.failureClass,
    frames:rows.length,firstFrame:edge(rows[0]),lastFrame:edge(rows.at(-1)),
    expectedFrames:frame?.expectedSourceIndices?.length,unknownRGBAFrames:frame?.actualSourceIndices?.filter(v=>v===null).length,
    actualFirstSourceIndices:frame?.actualSourceIndices?.slice(0,8),actualLastSourceIndices:frame?.actualSourceIndices?.slice(-8),
    frameQualified:row.frameConsumerQualified,directQualified:row.directQualification?.qualified,
    referenceComplete:row.referenceComplete,joinedBytesVerified:row.joinedBytesVerified,
    publicVideoSuffixQualified:row.publicVideoSuffixQualified,joinedSHA256:row.joinedSHA256,joinedDirectScope:row.joinedDirectScope,
    colorScope:row.colorScope,colorMetadata:row.colorMetadata,appendedMetadata:row.appendedMetadata,
    deliveredBytesVerified:row.deliveredBytesVerified,metadataByteIdentity:row.metadataByteIdentity,
    appendedBytesVerified:row.appendedBytesVerified,appends:row.appends,appendFailures:row.appendFailures,
    appendConfiguration:row.appendConfiguration,rawMSEClockFacts:row.rawMSEClockFacts,endOfStreamReturned:row.endOfStreamReturned,
    mediaSourceState:row.mediaSourceState,afterAppend:row.afterAppend,beforeSeek:row.beforeSeek,
    seekSetupQualification:row.seekSetupQualification,generatedSHA256:row.generatedSHA256,
    capabilities:row.capabilities,adapter:row.adapter,dataset:row.dataset,
    droppedCallbacks:phase?.droppedCallbacks,firstCallbackGap:phase?.firstCallbackGap,
    quality:phase?.quality,captureErrors:phase?.captureErrors,canvasAttributes:row.observer?.canvasAttributes,
    directReferenceBytesVerified:row.directReferenceBytesVerified,directReferenceBodies:row.directReferenceBodies,
    forceSeek:row.forceSeek,events:phase?.events,
    failures:Object.fromEntries(['pageErrors','snapshotFailure','httpOverflow','httpBodyFailure','directReferenceRouteMismatch',
      'publicAssetHashMismatch','directMediaRequested','wrongHLSRecipe','unexpectedSelectedAsset','selectedHLSError'].map(k=>[k,row[k]||false]))};
}
