import {readFile,writeFile} from 'node:fs/promises';
import {createHash} from 'node:crypto';
import {createReadStream} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {chromium} from '@playwright/test';
import {installBrowserObserver,frameQualification,consumerQualification,browserAudioDecode,completeAudioQualification} from './hls-nonkey-browser-observer.mjs';
const input=await readFile(process.argv[2]);if(input.length>65536)throw Error('private_input_bound');
const config=JSON.parse(input), result={cases:[],productionAcceptance:false,clientSourceChanged:false,
  browserAudioPresentationAccepted:false,reference:undefined};
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
let browserServer,browser,context,browserIdentity;
let terminated=false,settlePromise;
const groupMembers=id=>{
  const raw=execFileSync('ps',['-eo','pid=,pgid='],{encoding:'utf8',timeout:3000,maxBuffer:262144});
  return raw.split('\n').map(v=>v.trim().split(/\s+/)).filter(v=>v.length===2 && Number(v[1])===id).map(v=>Number(v[0]));
};
const identity=async pid=>{
  const raw=await readFile('/proc/'+pid+'/stat','utf8');
  if(raw.length>4096)throw Error('browser_identity_bound');
  const fields=raw.slice(raw.lastIndexOf(')')+2).trim().split(/\s+/);
  return {pid,parent:Number(fields[1]),group:Number(fields[2]),session:Number(fields[3]),startTicks:fields[19]};
};
const bounded=async(promise,milliseconds)=>{
  let timer;
  try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('cleanup_timeout')),milliseconds);})]);}
  finally{clearTimeout(timer);}
};
function settleBrowser(){
  if(settlePromise)return settlePromise;
  settlePromise=(async()=>{
  result.browserCleanupErrors=[];
  for(const close of [()=>context?.close(),()=>browser?.close(),()=>browserServer?.close()]){
    try{await bounded(Promise.resolve(close()),5000);}catch{result.browserCleanupErrors.push('browser_close_failed');}
  }
  if(browserServer && browserIdentity){
    try{
      if(groupMembers(browserIdentity.group).length)await bounded(browserServer.kill(),5000);
      let zeros=0;const deadline=Date.now()+3000;
      while(zeros<2 && Date.now()<deadline){
        const members=groupMembers(browserIdentity.group);
        zeros=members.length===0?zeros+1:0;
        await new Promise(resolve=>setTimeout(resolve,50));
      }
      result.ownedChromiumJoin={identity:browserIdentity,confirmedZeroSamples:zeros,
        remainingOwnedPIDs:groupMembers(browserIdentity.group)};
      if(zeros!==2)result.browserCleanupErrors.push('browser_group_not_joined');
    }catch{result.browserCleanupErrors.push('browser_group_observation_failed');}
  }
  if(result.browserCleanupErrors.length)result.result='failed';
  })();return settlePromise;
}
process.once('SIGTERM',()=>{terminated=true;if(browserServer && browserIdentity)void settleBrowser();});
const api=async(path,method='GET',body)=>{
  const response=await context.request.fetch(config.origin+path,{method,data:body,timeout:15000});
  if(!response.ok())throw Error('public_status_'+response.status());
  return response;
};
async function observe(id,request,label){
  const row={request,label,result:'observation-failed',http:[],failureClass:undefined};
  const expected=config.cases.find(v=>v.request===request);
  const mediaPending=[];
  row.observedSelectedAssets={};
  const page=await context.newPage();
  page.on('response',response=>{
    const url=new URL(response.url());
    if(url.origin!==config.origin)return;
    if(row.http.length>=1024){row.httpOverflow=true;return;}
    const path=url.pathname;
    if(label!=='source-reference' && path.startsWith('/media/'))row.directMediaRequested=true;
    if(label!=='source-reference' && path.startsWith('/hls/')){
      const prefix=expected.selectedSource.slice(0,expected.selectedSource.lastIndexOf('/')+1);
      if(!path.startsWith(prefix))row.wrongHLSRecipe=true;
      const name=path.slice(prefix.length);
      if(Object.hasOwn(expected.publicAssetSHA256,name)){
        mediaPending.push(response.body().then(bytes=>{
          if(bytes.length>2<<20)throw Error('public_browser_asset_bound');
          const actual={status:response.status(),sha256:sha(bytes),expectedSHA256:expected.publicAssetSHA256[name]};
          if(actual.status!==200 || actual.sha256!==actual.expectedSHA256)row.publicAssetHashMismatch=true;
          if(!row.observedSelectedAssets[name])row.observedSelectedAssets[name]=[];
          row.observedSelectedAssets[name].push(actual);
        }).catch(()=>{row.httpBodyFailure=true;}));
      }
    }
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
    row.currentStage='actual-media-decode';
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
    await page.locator('video').focus();
    row.playControl='unchanged-application-keyboard-K';
    if(await page.locator('video').evaluate(v=>v.paused))await page.keyboard.press('k');
    row.currentStage='complete-presented-EOF';
    await page.waitForFunction(()=>window.nonkeyObservation?.active?.ended,{},{timeout:85000});
    row.adapter=await page.locator('video').evaluate(v=>({blob:v.currentSrc.startsWith('blob:'),
      publicHLS:v.currentSrc.includes('/hls/'),direct:v.currentSrc.includes('/media/'),
      nativeHLS:v.canPlayType('application/vnd.apple.mpegurl'),hlsAvailable:typeof window.Hls!=='undefined'}));
    row.result='observed';row.currentStage='complete';
  }catch(error){row.failureClass=error.message?.match(/^[a-z_]+(?:_[0-9]+)?$/)?.[0]||error.name||'browser_operation';}
  finally{
    await Promise.all(mediaPending);
    try{row.observer=await page.evaluate(()=>window.nonkeySnapshot());}catch{row.snapshotFailure=true;}
    if(label!=='source-reference')row.selectedPublicHLSObserved=Boolean(!row.wrongHLSRecipe && !row.directMediaRequested &&
      !row.httpBodyFailure && expected?.initialDeliveryStable && Object.entries(expected.publicAssetSHA256).every(([name,digest])=>{
        const observed=row.observedSelectedAssets[name];return observed?.length>0 && observed.every(v=>v.status===200 && v.sha256===digest);
      }) && (row.adapter?.blob && row.adapter?.hlsAvailable || row.adapter?.publicHLS));
    await page.close();
  }
  return row;
}
try{
  const executable=chromium.executablePath();
  const binaryHash=createHash('sha256');let binaryBytes=0;
  for await(const part of createReadStream(executable)){
    binaryBytes+=part.length;if(binaryBytes>512<<20)throw Error('browser_binary_bound');binaryHash.update(part);
  }
  result.browserBinary={sha256:binaryHash.digest('hex'),bytes:binaryBytes,packageVersion:'@playwright/test1.63.0',
    launchExecutablePathMatched:true,retries:0,workers:1};
  browserServer=await chromium.launchServer({executablePath:executable,timeout:15000});
  browserIdentity=await identity(browserServer.process().pid);
  if(browserIdentity.parent!==process.pid || browserIdentity.group!==browserIdentity.pid ||
    browserIdentity.session!==browserIdentity.pid)throw Error('browser_owned_detached_identity');
  await writeFile(config.browserOwnerFile,JSON.stringify({...browserIdentity,nodePID:process.pid})+'\n',{mode:0o600});
  if(terminated)throw Error('handled_browser_termination');
  browser=await chromium.connect(browserServer.wsEndpoint(),{timeout:15000});
  result.browserVersion=browser.version();
  context=await browser.newContext({extraHTTPHeaders:{Authorization:'Bearer '+config.token},viewport:{width:960,height:720}});
  await context.addInitScript(installBrowserObserver);
  result.referenceScope='Same-browser complete coded AVC source; original source packet-copy remux, no lossless encoder claim';
  result.bundle={};
  for(const [name,path] of [['player','/static/player.js'],['hls','/static/hls.min.js']]){
    const bytes=await (await api(path)).body();if(bytes.length>4<<20)throw Error('bundle_bound');
    result.bundle[name]={sha256:sha(bytes),bytes:bytes.length};
  }
  result.reference=await observe(config.referenceID,0,'source-reference');
  const reference=result.reference.observer?.phases?.[0];
  result.referenceComplete=Boolean(reference?.ended && reference.rows.length===768 &&
    reference.droppedCallbacks===0 && reference.firstCallbackGap===0 &&
    reference.captureErrors.length===0 && reference.quality?.droppedVideoFrames===0 &&
    result.reference.result==='observed' && !result.reference.pageErrors && !reference.events.some(e=>e.errorCode>0 || e.name==='error'));
  for(const item of config.cases){
    if(terminated)throw Error('handled_browser_termination');
    for(const label of ['unchanged-client','forced-source-coordinate-seek']){
      const row=await observe(config.itemID,item.request,label);
      row.expectedSourceIndices=item.expectedSourceIndices;
      const selected=row.observer?.phases?.at(-1);
      if(selected && reference)row.frameQualification=frameQualification(selected,reference.rows,item.expectedSourceIndices);
      row.referenceComplete=result.referenceComplete;
      row.consumerQualification=consumerQualification(row,reference?.rows||[],item.expectedSourceIndices);
      row.frameConsumerQualified=row.consumerQualification.qualified;
      result.cases.push(row);
      console.log(JSON.stringify({request:row.request,label:row.label,result:row.result,currentStage:row.currentStage,
        failureClass:row.failureClass,frames:selected?.rows?.length,droppedCallbacks:selected?.droppedCallbacks,
        actualFirstSourceIndices:row.frameQualification?.actualSourceIndices?.slice(0,4),
        frameQualified:row.frameConsumerQualified,referenceComplete:row.referenceComplete}));
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
finally{await settleBrowser();
  const raw=JSON.stringify(result)+'\n';if(Buffer.byteLength(raw)>8<<20)throw Error('browser_receipt_bound');
  await writeFile(config.output,raw);
  console.log(JSON.stringify({result:result.result,referenceComplete:result.referenceComplete,
    cases:result.cases.map(r=>({request:r.request,label:r.label,result:r.result,failureClass:r.failureClass,
      frames:r.observer?.phases?.at(-1)?.rows?.length,frameQualified:r.frameConsumerQualified,
      audioQualified:r.qualification?.qualified,forceSeek:r.forceSeek})),
    browserAudioPresentationAccepted:false,productionAcceptance:false}));
}
if(result.result!=='observed')process.exitCode=1;
