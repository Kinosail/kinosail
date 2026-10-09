// Fixed, bounded pre-assertion observations. No HTML, URLs or arbitrary classes.
const kinds = ['player-settings', 'player-recovery', 'player-startup', 'home-resume', 'player-method-before-switch', 'player-method-after-switch', 'player-loading-width', 'keyboard-focus'];
const displays = ['none', 'block', 'inline', 'inline-block', 'grid', 'flex', 'inline-flex', 'contents', 'other', 'unavailable'];
const totals = new WeakMap();

export function responsiveFailureFacts(kind) {
  if (!['player-settings', 'player-recovery', 'player-startup', 'home-resume', 'player-method-before-switch', 'player-method-after-switch', 'player-loading-width', 'keyboard-focus'].includes(kind)) throw new Error('invalid responsive witness kind');
  const number = value => typeof value === 'number' && Number.isFinite(value) && Math.abs(value) <= 10000000 ? value : null;
  const dimension = value => number(value) !== null && value >= 0 ? value : null;
  const extended = ['player-method-before-switch','player-method-after-switch','player-loading-width','keyboard-focus'].includes(kind);
  const describe = element => {
    if (!element) return {available: false, rect: null, display: 'unavailable', hidden: null,...(extended?{connected:null,visibility:'unavailable'}:{})};
    const box = element.getBoundingClientRect(), style = getComputedStyle(element), display = style.display;
    return {available: true, rect: {x: number(box.x), y: number(box.y), width: dimension(box.width), height: dimension(box.height)},
      display: ['none', 'block', 'inline', 'inline-block', 'grid', 'flex', 'inline-flex', 'contents'].includes(display) ? display : 'other',
      hidden: Boolean(element.hidden),...(extended?{connected:typeof element.isConnected==='boolean'?element.isConnected:null,
        visibility:['visible','hidden','collapse'].includes(style.visibility)?style.visibility:'other'}:{})};
  };
  const base = {schemaVersion: 1, kind, viewport: {width: dimension(innerWidth), height: dimension(innerHeight)}};
  if (kind === 'keyboard-focus') {
    const skip=document.querySelector('a.skip[href="#main"]'),active=document.activeElement;
    const style=skip?getComputedStyle(skip):null;
    const activeControl=!active?'unavailable':active===skip?'skip':active===document.body?'body'
      :active.matches?.('[data-quick-connect-digit]')?'quick-code'
      :['INPUT','BUTTON','SELECT','TEXTAREA'].includes(active.tagName)?active.tagName.toLowerCase():'other';
    return {...base,elements:{skip:describe(skip)},state:{skipFocused:Boolean(skip && active===skip),
      skipFocusVisible:Boolean(skip?.matches?.(':focus-visible')),activeControl,activeAutofocus:Boolean(active?.hasAttribute?.('autofocus')),
      outlineWidth:dimension(parseFloat(style?.outlineWidth)),outlineStyle:['none','solid','dashed','dotted','double'].includes(style?.outlineStyle)?style.outlineStyle:'other',
      elapsedMs:dimension(globalThis.performance?.now?.()),documentReadyState:['loading','interactive','complete'].includes(document.readyState)?document.readyState:'unavailable'}};
  }
  if (kind === 'home-resume') {
    const featured = document.querySelector('.home-feature'), shelf = document.querySelector('.continue-shelf');
    const matches = element => {
      const text = element?.querySelector('h2,h3')?.textContent;
      return typeof text === 'string' && text.length <= 128 && text.trim() === 'Example Movie';
    };
    const cards = [...document.querySelectorAll('.continue-shelf article')];
    const continued = cards.slice(0, 64).find(matches);
    return {...base, elements: {featured: describe(featured), shelf: describe(shelf), continued: describe(continued)},
      state: {featuredMatchesExample: matches(featured), continuedMatchesExample: Boolean(continued),
        featuredProgressAvailable: Boolean(featured?.querySelector('[data-watch-progress]')),
        continuedProgressAvailable: Boolean(continued?.querySelector('[data-watch-progress]')), shelfScanTruncated: cards.length > 64}};
  }
  const settings = document.querySelector('.player-settings'), stage = document.querySelector('.media-stage');
  const options = document.querySelector('.player-native-options'), status = document.querySelector('[data-player-status]');
  const video = document.querySelector('video');
  const active = document.activeElement, agent = globalThis.navigator;
  const activeControl = !active ? 'unavailable' : active === document.body ? 'body'
    : active.matches?.('[data-player-settings-close]') ? 'close-settings'
    : active.matches?.('[data-subtitles]') ? 'subtitles'
    : ['INPUT', 'SELECT', 'TEXTAREA', 'BUTTON'].includes(active.tagName) ? active.tagName.toLowerCase() : 'other';
  const mediaNumber = value => number(value) !== null && value >= 0 ? value : null;
  const mediaBoolean = value => typeof value === 'boolean' ? value : null;
  const route=globalThis.location;
  let routeMode='other';
  if (route?.pathname?.length<=135 && /^\/watch\/[A-Za-z0-9_-]{1,128}$/.test(route.pathname) && (route.search??'').length<=128 && !route.hash) {
    routeMode=!route.search?'watch-default':route.search==='?compatible=1'?'watch-compatible':route.search==='?direct=1'?'watch-direct':'watch-other';
  }
  const epoch=globalThis.performance?.timeOrigin;
  const documentFacts={documentEpoch:typeof epoch==='number' && Number.isFinite(epoch) && epoch>=0 && epoch<=1e15?epoch:null,
    documentReadyState:['loading','interactive','complete'].includes(document.readyState)?document.readyState:'unavailable',routeMode,
    loopbackOwned:['http:','https:'].includes(route?.protocol) && ['localhost','127.0.0.1','[::1]'].includes(route?.hostname),
    touchPoints:Number.isInteger(agent?.maxTouchPoints) && agent.maxTouchPoints>=0 && agent.maxTouchPoints<=32?agent.maxTouchPoints:null,
    appleTouch:Boolean(/iPhone|iPad|iPod/.test(agent?.userAgent??'') || agent?.platform==='MacIntel' && agent.maxTouchPoints>1),
    statusState:['loading','buffering','seeking','ready','error'].includes(status?.dataset?.state)?status.dataset.state:'unavailable'};
  return {...base, elements: {settings: describe(settings), actions: describe(document.querySelector('.primary-player-actions:not([data-progress-notice])')),
    stage: describe(stage), nativeOptions: describe(options), status: describe(status)},
    state: {settingsInNativeOptions: Boolean(settings?.closest('.player-native-options')),
      settingsInStage: Boolean(settings?.closest('.media-stage')), stageHasSettings: Boolean(stage?.classList.contains('has-settings')),
      optionsHasSettings: Boolean(options?.classList.contains('has-settings')), statusRecovery: Boolean(status?.classList.contains('is-recovery')),
      activeControl, playerTheater: Boolean(document.body?.classList.contains('player-theater')),
      appleNativePlayback: video ? video.tagName === 'VIDEO'
        && (/iPhone|iPad|iPod/.test(agent?.userAgent ?? '') || agent?.platform === 'MacIntel' && agent.maxTouchPoints > 1)
        && typeof video.webkitEnterFullscreen === 'function' : null,
      videoAutoplayIntent: video ? Boolean(video.hasAttribute?.('autoplay') || video.hasAttribute?.('data-autoplay')) : null,
      videoStart: video?.getAttribute?.('data-start')?.match(/^(?:0|[1-9][0-9]{0,7})(?:\.[0-9]{1,6})?$/) ? mediaNumber(Number(video.getAttribute('data-start'))) : null,
      theaterAvailable: Boolean(document.querySelector('[data-theater]')),
      videoCurrentTime: mediaNumber(video?.currentTime), videoDuration: mediaNumber(video?.duration),
      videoNetworkState: Number.isInteger(video?.networkState) && video.networkState >= 0 && video.networkState <= 3 ? video.networkState : null,
      videoErrorCode: video && !video.error ? 0 : Number.isInteger(video?.error?.code) && video.error.code >= 0 && video.error.code <= 4 ? video.error.code : null,
      videoControls: mediaBoolean(video?.controls), videoAutoplay: mediaBoolean(video?.autoplay), videoEnded: mediaBoolean(video?.ended),
      videoPaused: typeof video?.paused === 'boolean' ? video.paused : null,
      videoReadyState: Number.isInteger(video?.readyState) && video.readyState >= 0 && video.readyState <= 4 ? video.readyState : null,
      ...(extended?documentFacts:{})}};
}

const keys = (value, expected) => value && typeof value === 'object' && !Array.isArray(value)
  && Object.keys(value).sort().join(',') === expected.slice().sort().join(',');
const number = value => value === null || typeof value === 'number' && Number.isFinite(value) && Math.abs(value) <= 10000000;
const dimension = value => number(value) && (value === null || value >= 0);
function valid(value, kind) {
  if (!keys(value, ['schemaVersion', 'kind', 'viewport', 'elements', 'state']) || value.schemaVersion !== 1 || value.kind !== kind
      || !keys(value.viewport, ['width', 'height']) || !Object.values(value.viewport).every(dimension)) return false;
  const extended=['player-method-before-switch','player-method-after-switch','player-loading-width','keyboard-focus'].includes(kind);
  const elements = kind === 'keyboard-focus'?['skip']:kind === 'home-resume' ? ['featured', 'shelf', 'continued'] : ['settings', 'actions', 'stage', 'nativeOptions', 'status'];
  if (!keys(value.elements, elements)) return false;
  for (const element of Object.values(value.elements)) {
    if (!keys(element, ['available', 'rect', 'display', 'hidden',...(extended?['connected','visibility']:[])]) || typeof element.available !== 'boolean'
        || !displays.includes(element.display) || !(element.hidden === null || typeof element.hidden === 'boolean')) return false;
    if(extended && (!(element.connected===null || typeof element.connected==='boolean') || !['unavailable','visible','hidden','collapse','other'].includes(element.visibility)))return false;
    if (!element.available) {
      if(extended && (element.connected!==null || element.visibility!=='unavailable'))return false;
      if (element.rect !== null || element.hidden !== null || element.display !== 'unavailable') return false;
    } else if (typeof element.hidden !== 'boolean' || element.display === 'unavailable'
        || !keys(element.rect, ['x', 'y', 'width', 'height']) || !Object.values(element.rect).every(number)
        || !dimension(element.rect.width) || !dimension(element.rect.height)) return false;
  }
  if(kind==='keyboard-focus') {
    return keys(value.state,['skipFocused','skipFocusVisible','activeControl','activeAutofocus','outlineWidth','outlineStyle','elapsedMs','documentReadyState'])
      && ['skipFocused','skipFocusVisible','activeAutofocus'].every(key=>typeof value.state[key]==='boolean')
      && ['unavailable','skip','body','quick-code','input','button','select','textarea','other'].includes(value.state.activeControl)
      && ['none','solid','dashed','dotted','double','other'].includes(value.state.outlineStyle)
      && dimension(value.state.outlineWidth) && dimension(value.state.elapsedMs)
      && ['loading','interactive','complete','unavailable'].includes(value.state.documentReadyState);
  }
  const states = kind === 'home-resume' ? ['featuredMatchesExample', 'continuedMatchesExample', 'featuredProgressAvailable', 'continuedProgressAvailable', 'shelfScanTruncated']
    : ['settingsInNativeOptions', 'settingsInStage', 'stageHasSettings', 'optionsHasSettings', 'statusRecovery', 'videoPaused', 'videoReadyState', 'activeControl', 'playerTheater', 'appleNativePlayback', 'videoCurrentTime', 'videoDuration', 'videoNetworkState', 'videoErrorCode', 'videoControls', 'videoAutoplay', 'videoEnded', 'videoAutoplayIntent', 'videoStart', 'theaterAvailable',...(extended?['documentEpoch','documentReadyState','routeMode','loopbackOwned','touchPoints','appleTouch','statusState']:[])];
  if (!keys(value.state, states)) return false;
  return Object.entries(value.state).every(([key, item]) => {
    if(key==='documentEpoch')return item===null || typeof item==='number' && Number.isFinite(item) && item>=0 && item<=1e15;
    if(key==='documentReadyState')return ['loading','interactive','complete','unavailable'].includes(item);
    if(key==='routeMode')return ['other','watch-default','watch-compatible','watch-direct','watch-other'].includes(item);
    if(key==='statusState')return ['loading','buffering','seeking','ready','error','unavailable'].includes(item);
    if(key==='touchPoints')return item===null || Number.isInteger(item) && item>=0 && item<=32;
    if (key === 'activeControl') return ['unavailable', 'body', 'close-settings', 'subtitles', 'input', 'select', 'textarea', 'button', 'other'].includes(item);
    if (['videoReadyState', 'videoErrorCode', 'videoNetworkState'].includes(key))
      return item === null || Number.isInteger(item) && item >= 0 && item <= (key === 'videoNetworkState' ? 3 : 4);
    if (['videoCurrentTime', 'videoDuration', 'videoStart'].includes(key)) return number(item) && (item === null || item >= 0);
    if (['videoPaused', 'videoControls', 'videoAutoplay', 'videoEnded', 'appleNativePlayback', 'videoAutoplayIntent'].includes(key)) return item === null || typeof item === 'boolean';
    return typeof item === 'boolean';
  });
}

export async function attachResponsiveFailure(page, info, kind) {
  if (!kinds.includes(kind)) throw new Error('invalid responsive witness kind');
  let body, timer;
  try {
    const value = await Promise.race([page.evaluate(responsiveFailureFacts, kind),
      new Promise((_, reject) => {timer = setTimeout(() => reject(new Error("observation unavailable")), 1000);})]);
    body = valid(value, kind) ? JSON.stringify(value) : JSON.stringify({schemaVersion: 1, kind, unavailable: true, reason: 'invalid_snapshot'});
  } catch {body = JSON.stringify({schemaVersion: 1, kind, unavailable: true, reason: 'snapshot_failed'});}
  finally {clearTimeout(timer);}
  const bytes = Buffer.byteLength(body), used = totals.get(info) ?? 0;
  if (bytes > 16384 || used + bytes > 65536) return false;
  totals.set(info, used + bytes);
  try {
    await Promise.race([info.attach('responsive-' + kind, {body, contentType: 'application/json'}),
      new Promise((_, reject) => {timer = setTimeout(() => reject(new Error('attachment unavailable')), 1000);})]);
    return true;
  } catch {return false;} // Observation failure must not replace the existing assertion.
  finally {clearTimeout(timer);}
}
