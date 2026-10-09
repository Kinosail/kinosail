// The routed startup fixture observes delivery, not application consumption.
export async function holdStartupMedia(page, source, target) {
  if (!['direct', 'compatible', 'automatic'].includes(source)) throw new Error('invalid startup source');
  let ownedOrigin, ownedItem;
  if (arguments.length >= 3) {
    const fields = target && typeof target === 'object' && !Array.isArray(target) ? Object.getOwnPropertyDescriptors(target) : {};
    if (source !== 'direct' || Reflect.ownKeys(fields).sort().join(',') !== 'baseURL,watch'
        || !['baseURL','watch'].every(key => Object.hasOwn(fields[key], 'value'))
        || typeof fields.baseURL.value !== 'string' || fields.baseURL.value.length > 2048
        || typeof fields.watch.value !== 'string' || fields.watch.value.length > 135
        || !/^\/watch\/[A-Za-z0-9_-]{1,128}$/.test(fields.watch.value)) throw Error('invalid loading target');
    let base;
    try {base = new URL(fields.baseURL.value);} catch {throw Error('invalid loading target');}
    if (!['http:','https:'].includes(base.protocol) || !['localhost','127.0.0.1','[::1]'].includes(base.hostname)
        || base.username || base.password || base.pathname !== '/' || base.search || base.hash
        || ![base.origin, base.origin + '/'].includes(fields.baseURL.value)) throw Error('invalid loading target');
    ownedOrigin = base.origin; ownedItem = fields.watch.value.slice(7);
  }
  const match = kind => ownedOrigin ? url => url.href.length <= 4096 && url.origin === ownedOrigin && !url.hash
    && (kind === 'media' ? url.pathname === '/media/' + ownedItem
      : url.pathname.startsWith('/hls/' + ownedItem + '/') && /^\/hls\/[A-Za-z0-9_-]{1,128}\/(?:[A-Za-z0-9_.-]+\/)*[A-Za-z0-9_.-]+$/.test(url.pathname))
    : '**/' + kind + '/**';
  const state = {mediaEntered: 0, hlsEntered: 0, released: false, finished: 0, failed: 0, overflow: false};
  let release, timer;
  const ready = new Promise(resolve => {release = resolve;});
  const tasks = new Set();
  const count = key => {if (state[key] >= 64) state.overflow = true; else state[key]++;};
  const handler = kind => route => {
    if (tasks.size >= 64) {state.overflow = true; throw new Error('startup hold capacity');}
    count(kind === 'media' ? 'mediaEntered' : 'hlsEntered');
    const task = (async () => {
      await ready;
      try {
        if (kind === 'media' && source === 'automatic') await route.fulfill({status: 206,
          contentType: 'video/mp4', headers: {'Content-Range': 'bytes 0-0/1'}, body: 'x'});
        else await route.continue();
        count('finished');
      } catch (error) {count('failed'); throw error;}
    })();
    tasks.add(task);
    task.then(() => tasks.delete(task), () => tasks.delete(task));
    return task;
  };
  const media = handler('media'), hls = handler('hls'), mediaMatch = match('media'), hlsMatch = match('hls');
  await page.route(mediaMatch, media);
  try {await page.route(hlsMatch, hls);} catch (error) {
    release(); await page.unroute(mediaMatch, media); throw error;
  }
  return {
    snapshot: () => ({...state}),
    release: () => {state.released = true; release();},
    attach: async (info, phase) => {
      if (!['opening', 'pending', 'readiness', 'play-control', 'gesture', 'seek'].includes(phase)) throw new Error('invalid startup phase');
      try {
        return await Promise.race([info.attach('startup-hold.json', {body: Buffer.from(JSON.stringify({version: 1, phase, ...state})),
          contentType: 'application/json'}).then(() => true),
          new Promise(resolve => {timer = setTimeout(() => resolve(false), 500);})]);
      } catch {return false;} finally {clearTimeout(timer);}
    },
    close: async () => {
      state.released = true; release();
      try {
        return await Promise.race([(async () => {
          await page.unroute(mediaMatch, media);
          await page.unroute(hlsMatch, hls);
          await Promise.allSettled([...tasks]); return true;
        })(),
          new Promise(resolve => {timer = setTimeout(() => resolve(false), 1000);})]);
      } finally {clearTimeout(timer);}
    }
  };
}
