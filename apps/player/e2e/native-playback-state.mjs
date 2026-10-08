// Closed observations of the existing native smoke video; never attach source URLs or SDK errors.
export async function attachNativePlaybackState(video, info, phase) {
  if (!['before-click', 'after-click', 'failure'].includes(phase)) throw Error('invalid native playback phase');
  let timer;
  try {
    const media = await Promise.race([video.evaluate(element => ({
      paused: element.paused, ended: element.ended, seeking: element.seeking,
      currentTime: Number.isFinite(element.currentTime) ? element.currentTime : null,
      duration: Number.isFinite(element.duration) ? element.duration : null,
      readyState: element.readyState, networkState: element.networkState, errorCode: element.error ? element.error.code : 0,
      controls: element.controls, nativeControls: element.hasAttribute('data-native-controls'),
    })).catch(() => null), new Promise(resolve => {timer = setTimeout(() => resolve(null), 500);})]);
    clearTimeout(timer);
    const valid = media && Object.keys(media).sort().join(',') === 'controls,currentTime,duration,ended,errorCode,nativeControls,networkState,paused,readyState,seeking'
      && ['paused', 'ended', 'seeking', 'controls', 'nativeControls'].every(key => typeof media[key] === 'boolean')
      && ['currentTime', 'duration'].every(key => media[key] === null || typeof media[key] === 'number' && Number.isFinite(media[key]) && media[key] >= 0 && media[key] <= 31622400)
      && [['readyState', 4], ['networkState', 3], ['errorCode', 4]].every(([key, maximum]) => Number.isInteger(media[key]) && media[key] >= 0 && media[key] <= maximum);
    const body = JSON.stringify(valid ? {schemaVersion: 1, phase, media} : {schemaVersion: 1, phase, unavailable: true});
    if (Buffer.byteLength(body) > 2048) return;
    await Promise.race([info.attach('native-playback-state', {body, contentType: 'application/json'}),
      new Promise(resolve => {timer = setTimeout(resolve, 500);})]);
  } catch { /* Observation cannot replace the existing action or assertion failure. */ }
  finally {clearTimeout(timer);}
}
