// Read only public media/control state. Missing or stalled renderer evidence is
// unavailable; observation must not change the theater actions or stable gate.
export async function captureTheaterState(page) {
  let timer, value;
  try {
    value = await Promise.race([
      page.evaluate(() => {
        const video = document.querySelector('video');
        return {documentReady: document.readyState, paused: video?.paused, ended: video?.ended,
          readyState: video?.readyState, errorCode: video?.error?.code ?? (video ? 0 : undefined), currentTime: video?.currentTime,
          theaterActive: document.body.classList.contains('player-theater'),
          theaterPressed: document.querySelector('[data-theater]')?.getAttribute('aria-pressed'),
          toolbarHidden: document.querySelector('.player-stage-toolbar')?.hidden};
      }),
      new Promise(resolve => {timer = setTimeout(resolve, 500);}),
    ]);
  } catch { /* Missing evidence is not a theater result. */ }
  finally {clearTimeout(timer);}
  const state = value && typeof value === 'object' ? value : {};
  const flag = name => typeof state[name] === 'boolean' ? state[name] : 'unavailable';
  return {documentReady: ['loading', 'interactive', 'complete'].includes(state.documentReady) ? state.documentReady : 'unavailable',
    paused: flag('paused'), ended: flag('ended'),
    readyState: Number.isInteger(state.readyState) && state.readyState >= 0 && state.readyState <= 4 ? state.readyState : 'unavailable',
    errorCode: Number.isInteger(state.errorCode) && state.errorCode >= 0 && state.errorCode <= 4 ? state.errorCode : 'unavailable',
    currentTime: Number.isFinite(state.currentTime) && state.currentTime >= 0 && state.currentTime <= 31536000 ? state.currentTime : 'unavailable',
    theaterActive: flag('theaterActive'), theaterPressed: ['true', 'false'].includes(state.theaterPressed) ? state.theaterPressed : 'unavailable',
    toolbarHidden: flag('toolbarHidden')};
}

export function parseControlMarker(text) {
  if(typeof text!=="string"||text.length>1024||!text.startsWith("kinosail-theater-control "))return;
  const raw=text.slice(24), keys=["event","label","paused","ready","network","position","visible","focusVisible"];
  if(raw.includes("\\"))return;
  try {
    const value=JSON.parse(raw), fields=[...raw.matchAll(/"([^"\\]+)"\s*:/g)].map(match=>match[1]);
    if(!value||typeof value!=="object"||Array.isArray(value)||fields.length!==8||new Set(fields).size!==8||Object.keys(value).length!==8||fields.some(key=>!keys.includes(key)))return;
    if(!["toggle-capture","theater-capture"].includes(value.event)||!["Play","Pause","Theater","Exit theater"].includes(value.label)||typeof value.paused!=="boolean"||typeof value.focusVisible!=="boolean"||!Number.isInteger(value.ready)||value.ready<0||value.ready>4||!Number.isInteger(value.network)||value.network<0||value.network>3||!Number.isFinite(value.position)||value.position<0||value.position>31536000||!["visible","hidden","prerender"].includes(value.visible))return;
    if(!(value.event==="toggle-capture"?["Play","Pause"]:["Theater","Exit theater"]).includes(value.label))return;
    return {...value,source:"unverified-console"};
  } catch {}
}

export async function installPlaybackObservation(page,navigation,records,owned={}) {
  navigation.observePlayback();
  const started=performance.now();
  const listener=message=>{try{const value=parseControlMarker(message.text());if(value&&records.length<64)records.push({...value,timeMs:Math.min(600000,Math.max(0,Math.round(performance.now()-started)))});}catch{}};
  page.on("console",listener);
  owned.stop=()=>page.off("console",listener);
  try {
    await page.addInitScript(()=>{
      const guard=Symbol.for("kinosail:qa:theater-observation");if(window[guard])return;window[guard]=true;
      let count=0;
      const media=event=>{
        const video=document.querySelector("video");if(!video||count>=64)return;count++;
        console.debug("kinosail-playback-lifecycle",JSON.stringify({event,source:video.currentSrc.startsWith("blob:")?"blob:":"/media",visible:document.visibilityState,position:video.currentTime,ready:video.readyState,network:video.networkState,paused:video.paused,pip:document.pictureInPictureElement===video||video.webkitPresentationMode==="picture-in-picture"}));
      };
      for(const event of ["play","pause","playing","loadedmetadata","emptied","error","seeking","seeked"])
        document.addEventListener(event,value=>{if(value.target instanceof HTMLVideoElement)media(event);},{capture:true});
      document.addEventListener("click",event=>{
        const button=event.target.closest?.("[data-player-toggle],[data-theater]");const video=document.querySelector("video");if(!button||!video||count>=64)return;
        const label=button.getAttribute("aria-label");if(!["Play","Pause","Theater","Exit theater"].includes(label))return;count++;
        console.debug("kinosail-theater-control",JSON.stringify({event:button.hasAttribute("data-theater")?"theater-capture":"toggle-capture",label,paused:video.paused,ready:video.readyState,network:video.networkState,position:video.currentTime,visible:document.visibilityState,focusVisible:button.matches(":focus-visible")}));
      },{capture:true});
    });
  } catch(error){page.off("console",listener);throw error;}
  return ()=>page.off("console",listener);
}

export async function observedTheaterFlow(page,navigation,records,operation) {
  const owned={};let timer;
  try {
    try {await Promise.race([installPlaybackObservation(page,navigation,records,owned),new Promise(resolve=>{timer=setTimeout(resolve,500);})]);}
    catch { /* Private observation cannot replace the original operation. */ }
    finally {clearTimeout(timer);}
    return await operation();
  } finally {owned.stop?.();}
}
