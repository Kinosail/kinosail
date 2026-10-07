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
