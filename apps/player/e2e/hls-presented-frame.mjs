// Self-contained for Playwright evaluate: actual RVFC pixels, never inferred
// from the media clock. Fixed fixture code: ten bits, complement row, two guards.
export function firstPresentedFrame(value) {
 const object=v=>v!==null&&typeof v==='object'&&!Array.isArray(v);
 const keys=(v,names)=>object(v)&&Object.keys(v).sort().join(',')===[...names].sort().join(',');
 const finite=(v,max)=>typeof v==='number'&&Number.isFinite(v)&&v>=0&&v<=max;
 if(!keys(value,['schema','prepared','committed','failure','frames','events'])||value.schema!==1||value.prepared!==true
  ||value.committed!==true||value.failure!==null||!Array.isArray(value.frames)||value.frames.length<1||value.frames.length>64
  ||!Array.isArray(value.events)||value.events.length>32)throw new Error('incomplete presented frame observation');
 for(const event of value.events){
  if(!keys(event,['kind','afterCommit','currentTime'])||!['seeking','seeked','waiting','playing','pause','emptied','loadedmetadata','error'].includes(event.kind)
   ||typeof event.afterCommit!=='boolean'||!finite(event.currentTime,60))throw new Error('invalid presented media event');
 }
 let first;
 for(const frame of value.frames){
  if(!keys(frame,['afterCommit','frameIndex','mediaTime','currentTime','presentedFrames','paused','seeking','pending','visible'])
   ||!Number.isInteger(frame.frameIndex)||frame.frameIndex<0||frame.frameIndex>767
   ||!finite(frame.mediaTime,60)||!finite(frame.currentTime,60)||!Number.isInteger(frame.presentedFrames)
   ||!finite(frame.presentedFrames,1000000)||!['afterCommit','paused','seeking','pending','visible'].every(k=>typeof frame[k]==='boolean'))
   throw new Error('invalid presented frame observation');
  if(!frame.afterCommit||!frame.visible)continue;
  if(frame.frameIndex>=720){if(!frame.pending)throw new Error('old window remained visible after seek');continue;}
  first ||= frame;
 }
 if(!first||first.frameIndex!==300)throw new Error('first visible new seek frame was not source frame 300');
 return first;
}
export function installPresentedFrames() {
 let video,callback,stopped=false,prepared=false,committed=false,failure=null;
 const frames=[],events=[],listeners=[];
 const number=(value,max)=>typeof value==='number'&&Number.isFinite(value)&&value>=0&&value<=max;
 const fail=kind=>{failure ||= kind;};
 const on=(owner,name,fn,capture=false)=>{owner.addEventListener(name,fn,capture);listeners.push([owner,name,fn,capture]);};
 const snapshot=()=>({schema:1,prepared,committed,failure,frames:frames.map(v=>({...v})),events:events.map(v=>({...v}))});
 function code(context) {
  const shade=(x,y)=>{
   const p=context.getImageData(x,y,1,1).data;
   if(p.length!==4||p[3]!==255||!Array.from(p).every(v=>Number.isInteger(v)&&v>=0&&v<=255)
      ||Math.max(...p.slice(0,3))-Math.min(...p.slice(0,3))>12)throw new Error('code');
   if(p[0]<=48&&p[1]<=48&&p[2]<=48)return 0;
   if(p[0]>=208&&p[1]>=208&&p[2]>=208)return 1;
   throw new Error('code');
  };
  if(shade(200,24)!==0||shade(232,24)!==1)throw new Error('code');
  let value=0;
  for(let i=0;i<10;i++){
   const bit=shade(24+16*i,24);
   if(shade(24+16*i,56)!==1-bit)throw new Error('code');
   value+=bit*2**i;
  }
  if(value>767)throw new Error('code');
  return value;
 }
 const sample=(now,metadata)=>{
  if(stopped)return;
  if(prepared){
   if(frames.length>=64){fail('frame-overflow');return;}
   let frameIndex=null;
   if(!number(video.videoWidth,1280)||video.videoWidth<160||!number(video.videoHeight,720)||video.videoHeight<90
      ||!number(metadata.mediaTime,60)||!number(video.currentTime,60)||!number(metadata.presentedFrames,1000000)
      ||!Number.isInteger(metadata.presentedFrames)||!number(now,3600000)){fail('pixel-unavailable');}
   else{
    try{
     const canvas=document.createElement('canvas');canvas.width=640;canvas.height=360;
     const context=canvas.getContext('2d',{willReadFrequently:true});
     if(!context)throw new Error('canvas');
     context.drawImage(video,0,0,640,360);
     try{frameIndex=code(context);}catch{fail('pixel-code');}
    }catch{fail('pixel-unavailable');}
   }
   const bounds=video.getBoundingClientRect(),style=getComputedStyle(video);
   frames.push({afterCommit:committed,frameIndex,mediaTime:number(metadata.mediaTime,60)?metadata.mediaTime:null,
    currentTime:number(video.currentTime,60)?video.currentTime:null,presentedFrames:number(metadata.presentedFrames,1000000)?metadata.presentedFrames:null,
    paused:video.paused===true,seeking:video.seeking===true,
    pending:video.closest('.media-stage')?.classList.contains('is-busy')===true,
    visible:number(bounds.width,10000)&&bounds.width>0&&number(bounds.height,10000)&&bounds.height>0
      &&style.display!=='none'&&style.visibility==='visible'&&style.opacity==='1'});
  }
  callback=video.requestVideoFrameCallback(sample);
 };
 const bind=()=>{
  video=document.querySelector('video');
  if(!video||typeof video.requestVideoFrameCallback!=='function'){fail('frame-unavailable');return;}
  on(document,'change',event=>{
   if(prepared&&event.isTrusted&&event.target?.matches?.('[data-player-seek]')&&event.target.value==='12.5')committed=true;
  },true);
  for(const name of ['seeking','seeked','waiting','playing','pause','emptied','loadedmetadata','error']){
   on(video,name,()=>{if(prepared&&events.length<32)events.push({kind:name,afterCommit:committed,
    currentTime:number(video.currentTime,60)?video.currentTime:null});});
  }
  callback=video.requestVideoFrameCallback(sample);
 };
 const api={
  prepare(){if(!video||stopped||failure)throw new Error('presented frame observer unavailable');prepared=true;committed=false;frames.length=0;events.length=0;},
  snapshot,
  stop(){if(stopped)return;stopped=true;if(video&&callback!==undefined)video.cancelVideoFrameCallback(callback);
   for(const [owner,name,fn,capture]of listeners)owner.removeEventListener(name,fn,capture);listeners.length=0;}
 };
 window.hlsPresented=api;
 if(document.readyState==='loading')on(document,'DOMContentLoaded',bind);else bind();
 return api;
}
