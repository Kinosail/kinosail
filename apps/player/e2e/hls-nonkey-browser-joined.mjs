// Isolated decoder control using sealed bytes already delivered by the actual public server.
import {createHash} from 'node:crypto';
import {joinedFrameQualification} from './hls-nonkey-browser-observer.mjs';
export async function observeJoinedPublic(context,origin,item,bytes,reference,label,referenceComplete,retainedCases){
  const row={request:item.request,label,result:'observation-failed',referenceComplete,
    joinedDirectScope:'Blob direct decode of sealed actual public init plus every delivered cut; application client unchanged',
    publicVideoSuffixQualified:item.publicVideoSuffixQualified===true};
  retainedCases.push(row);
  const digest=createHash('sha256').update(bytes).digest('hex');
  if(!bytes.length || bytes.length>8<<20 || digest!==item.publicJoinedSHA256)throw Error('joined_public_byte_binding');
  row.joinedSHA256=digest;row.joinedBytesVerified=true;
  const page=await context.newPage();
  page.on('pageerror',()=>{row.pageErrors=(row.pageErrors||0)+1;});
  try{
    await page.goto(origin+'/healthz',{waitUntil:'domcontentloaded',timeout:15000});
    await page.evaluate(async bytes=>{
      const video=document.createElement('video');video.controls=true;
      document.body.append(video);await Promise.resolve();
      window.nonkeyJoinedObjectURL=URL.createObjectURL(new Blob([Uint8Array.from(bytes)],{type:'video/mp4'}));
      video.src=window.nonkeyJoinedObjectURL;
      const button=document.createElement('button');button.id='nonkey-joined-play';button.textContent='Play diagnostic';
      button.onclick=()=>{video.play().catch(()=>{window.nonkeyJoinedPlayFailed=true;});};
      document.body.append(button);
    },[...bytes]);
    row.currentStage='joined-direct-metadata';
    await page.waitForFunction(()=>document.querySelector('video')?.readyState>=2,{},{timeout:20000});
    if(label==='joined-direct-explicit-zero'){
      row.forceSeek=await page.evaluate(async()=>{
        const video=document.querySelector('video');
        window.nonkeyBeginPhase('joined-direct-explicit-zero');
        let seeking=false,seeked=false;
        const first=()=>{seeking=true;},last=()=>{seeked=true;};
        video.addEventListener('seeking',first);video.addEventListener('seeked',last);
        try{
          await new Promise(resolve=>{
            const timer=setTimeout(resolve,5000);
            video.addEventListener('seeked',()=>{clearTimeout(timer);resolve();},{once:true});
            video.currentTime=0;
          });
        }finally{video.removeEventListener('seeking',first);video.removeEventListener('seeked',last);}
        return {requestedLocal:0,seeking,seeked,rawTime:video.currentTime,
          diagnosticOnly:true,commonManagedSetterInvoked:false};
      });
    }
    row.currentStage='joined-direct-complete-EOF';
    await page.locator('#nonkey-joined-play').click({timeout:10000});
    await page.waitForFunction(()=>window.nonkeyJoinedPlayFailed || window.nonkeyObservation?.active?.ended,{},{timeout:85000});
    if(await page.evaluate(()=>window.nonkeyJoinedPlayFailed===true))throw Error('joined_direct_play_failed');
    row.adapter=await page.locator('video').evaluate(v=>({blob:v.currentSrc.startsWith('blob:'),direct:false,
      componentDirectDecode:true,rawTime:v.currentTime,rawDuration:v.duration}));
    row.result='observed';row.currentStage='complete';
  }catch(error){row.failureClass=error.message?.match(/^[a-z_]+$/)?.[0]||error.name||'joined_browser_operation';}
  finally{
    try{row.observer=await page.evaluate(()=>window.nonkeySnapshot());}catch{row.snapshotFailure=true;}
    try{await page.evaluate(()=>{document.querySelector('video')?.pause();
      if(window.nonkeyJoinedObjectURL)URL.revokeObjectURL(window.nonkeyJoinedObjectURL);});}
    finally{await page.close();}
  }
  row.frameQualification=joinedFrameQualification(row,reference,item.expectedSourceIndices);
  row.frameConsumerQualified=row.frameQualification.qualified;
  return row;
}
