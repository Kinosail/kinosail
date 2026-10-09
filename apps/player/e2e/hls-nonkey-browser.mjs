import {readFile,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {chromium} from '@playwright/test';
import {installBrowserObserver,frameQualification,browserAudioDecode,completeAudioQualification} from './hls-nonkey-browser-observer.mjs';
const input=await readFile(process.argv[2]);if(input.length>65536)throw Error('private_input_bound');
const config=JSON.parse(input), result={cases:[],productionAcceptance:false,clientSourceChanged:false,
  browserAudioPresentationAccepted:false,reference:undefined};
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const browser=await chromium.launch();
result.browserVersion=browser.version();
const context=await browser.newContext({extraHTTPHeaders:{Authorization:'Bearer '+config.token},viewport:{width:960,height:720}});
await context.addInitScript(installBrowserObserver);
const api=async(path,method='GET',body)=>{
  const response=await context.request.fetch(config.origin+path,{method,data:body,timeout:15000});
  if(!response.ok())throw Error('public_status_'+response.status());
  return response;
};
async function observe(id,request,label){
  const row={request,label,result:'observation-failed',http:[],failureClass:undefined};
  const page=await context.newPage();
  page.on('response',response=>{
    const url=new URL(response.url());
    if(url.origin!==config.origin || row.http.length>=1024)return;
    const path=url.pathname;
    const kind=path.endsWith('.m4s')?'fragment':path.endsWith('init.mp4')?'init':path.endsWith('.m3u8')?'playlist':
      path==='/static/player.js'?'player-bundle':path.startsWith('/api/v1/')?'api':path.startsWith('/watch/')?'watch':'other';
    row.http.push({kind,status:response.status()});
  });
  page.on('pageerror',()=>{row.pageErrors=(row.pageErrors||0)+1;});
  try{
    await api('/api/v1/items/'+id+'/progress','PUT',{seconds:request});
    const destination=config.origin+'/watch/'+id+(label==='source-reference'?'?direct=1':'?compatible=1');
    const response=await page.goto(destination,{waitUntil:'domcontentloaded',timeout:15000});
    if(response.status()!==200)throw Error('watch_not_public_200');
    row.capabilities=await page.evaluate(()=>({h264File:document.createElement('video').canPlayType('video/mp4; codecs="avc1.64001e"'),
      aacFile:document.createElement('audio').canPlayType('audio/mp4; codecs="mp4a.40.2"'),
      h264MSE:MediaSource.isTypeSupported('video/mp4; codecs="avc1.64001e"'),
      aacMSE:MediaSource.isTypeSupported('audio/mp4; codecs="mp4a.40.2"')}));
    await page.waitForFunction(()=>document.querySelector('video')?.readyState>=2,{},{timeout:20000});
    row.dataset=await page.locator('video').evaluate(v=>({start:v.dataset.start,duration:v.dataset.duration,
      compatibilityMode:v.dataset.compatibilityMode,policy:v.dataset.playbackPolicy,
      buffered:Array.from({length:v.buffered.length},(_,n)=>[v.buffered.start(n),v.buffered.end(n)]),
      seekable:Array.from({length:v.seekable.length},(_,n)=>[v.seekable.start(n),v.seekable.end(n)]),
      rawTime:Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype,'currentTime').get.call(v),
      projectedTime:v.currentTime,nativeHLS:v.canPlayType('application/vnd.apple.mpegurl'),
      hlsAvailable:typeof window.Hls!=='undefined'}));
    if(label==='forced-source-coordinate-seek'){
      row.forceSeek=await page.evaluate(async target=>{
        const v=document.querySelector('video');v.pause();
        let seeking=false,seeked=false;
        const first=()=>{seeking=true;},last=()=>{seeked=true;};
        v.addEventListener('seeking',first,{once:true});v.addEventListener('seeked',last,{once:true});
        v.currentTime=target;
        await new Promise(resolve=>{const end=setTimeout(resolve,5000);
          v.addEventListener('seeked',()=>{clearTimeout(end);resolve();},{once:true});});
        window.nonkeyBeginPhase('after-explicit-source-coordinate-seek');
        return {requestedSourceCoordinate:target,seeking,seeked,projectedTime:v.currentTime,
          rawTime:Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype,'currentTime').get.call(v),
          diagnosticOnly:true,commonManagedSetterInvoked:false};
      },request);
    }
    await page.locator('video').click({force:true});
    await page.locator('video').evaluate(v=>{v.playbackRate=0.5;return v.play();});
    await page.waitForFunction(()=>window.nonkeyObservation?.active?.ended,{},{timeout:85000});
    row.result='observed';
  }catch(error){row.failureClass=error.message?.match(/^[a-z_]+(?:_[0-9]+)?$/)?.[0]||error.name||'browser_operation';}
  finally{
    try{row.observer=await page.evaluate(()=>window.nonkeySnapshot());}catch{row.snapshotFailure=true;}
    await page.close();
  }
  return row;
}
try{
  result.bundle={};
  for(const [name,path] of [['player','/static/player.js'],['hls','/static/hls.min.js']]){
    const bytes=await (await api(path)).body();if(bytes.length>4<<20)throw Error('bundle_bound');
    result.bundle[name]={sha256:sha(bytes),bytes:bytes.length};
  }
  result.reference=await observe(config.referenceID,0,'source-reference');
  const reference=result.reference.observer?.phases?.[0];
  result.referenceComplete=Boolean(reference?.ended && reference.rows.length===768 &&
    reference.droppedCallbacks===0 && reference.captureErrors.length===0);
  for(const item of config.cases){
    for(const label of ['unchanged-client','forced-source-coordinate-seek']){
      const row=await observe(config.itemID,item.request,label);
      row.expectedSourceIndices=item.expectedSourceIndices;
      const selected=row.observer?.phases?.at(-1);
      if(selected && reference)row.frameQualification=frameQualification(selected,reference.rows,item.expectedSourceIndices);
      row.referenceComplete=result.referenceComplete;
      row.frameConsumerQualified=Boolean(result.referenceComplete && row.frameQualification?.qualified &&
        (label!=='forced-source-coordinate-seek' || row.forceSeek?.seeking && row.forceSeek?.seeked));
      result.cases.push(row);
    }
    const bytes=await readFile(item.publicJoinedPath);
    if(bytes.length>8<<20)throw Error('public_joined_bound');
    const page=await context.newPage();await page.goto(config.origin+'/healthz');
    const audio={request:item.request,publicAssetSHA256:sha(bytes),scope:'Complete browser AudioContext decode diagnostic'};
    try{
      audio.actual=await page.evaluate(browserAudioDecode,[...bytes]);
      audio.qualification=completeAudioQualification(audio.actual,item.expectedPCM);
    }catch(error){audio.failureClass=error.name||'browser_audio_decode';}
    finally{await page.close();}
    result.cases.push(audio);
  }
  result.result='observed';
}catch(error){result.result='failed';result.failureClass=error.message?.match(/^[a-z_]+$/)?.[0]||error.name;}
finally{await context.close();await browser.close();
  const raw=JSON.stringify(result)+'\n';if(Buffer.byteLength(raw)>8<<20)throw Error('browser_receipt_bound');
  await writeFile(config.output,raw);
  console.log(JSON.stringify({result:result.result,referenceComplete:result.referenceComplete,
    cases:result.cases.map(r=>({request:r.request,label:r.label,result:r.result,failureClass:r.failureClass,
      frames:r.observer?.phases?.at(-1)?.rows?.length,frameQualified:r.frameConsumerQualified,
      audioQualified:r.qualification?.qualified,forceSeek:r.forceSeek})),
    browserAudioPresentationAccepted:false,productionAcceptance:false}));
}
if(result.result!=='observed')process.exitCode=1;
