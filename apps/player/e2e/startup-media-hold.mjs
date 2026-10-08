// The routed startup fixture observes delivery, not application consumption.
export async function holdStartupMedia(page, source) {
  if (!['direct', 'compatible', 'automatic'].includes(source)) throw new Error('invalid startup source');
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
  const media = handler('media'), hls = handler('hls');
  await page.route('**/media/**', media);
  try {await page.route('**/hls/**', hls);} catch (error) {
    release(); await page.unroute('**/media/**', media); throw error;
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
          await page.unroute('**/media/**', media);
          await page.unroute('**/hls/**', hls);
          await Promise.allSettled([...tasks]); return true;
        })(),
          new Promise(resolve => {timer = setTimeout(() => resolve(false), 1000);})]);
      } finally {clearTimeout(timer);}
    }
  };
}
