// Read-only public item progress. Never attach an item ID, title, URL or raw response.
export async function attachPlaybackState(page, info, watch, stage) {
  if (typeof watch !== 'string' || watch.length > 135 || !/^\/watch\/[A-Za-z0-9_-]{1,128}$/.test(watch)
      || !['before-method', 'before-loading', 'at-method-switch', 'after-method-switch'].includes(stage)) throw Error('invalid playback witness');
  const value = {schemaVersion: 1, stage, status: null, unavailable: true};
  let timer;
  try {
    const facts = await Promise.race([(async () => {
      const response = await page.request.get('/api/v1/items/' + watch.slice(7), {timeout: 1000});
      const status = response.status();
      if (!Number.isInteger(status) || status < 100 || status > 599) return value;
      const closed = {...value, status};
      if (status !== 200) return closed;
      const raw = await response.text();
      if (typeof raw !== 'string' || Buffer.byteLength(raw) > 65536) return closed;
      let progress;
      try {
        const parsed = JSON.parse(raw), stack = [];
        // JSON.parse accepts duplicate keys; reject ambiguity before projection.
        for (const token of raw.matchAll(/"(?:[^"\\]|\\.)*"|[{}\[\]]/g)) {
          const text = token[0];
          if (text === '{') stack.push(new Set());
          else if (text === '[') stack.push(null);
          else if (text === '}' || text === ']') stack.pop();
          else if (raw.slice(token.index + text.length).trimStart().startsWith(':')) {
            const key = JSON.parse(text), keys = stack.at(-1);
            if (keys.has(key)) return closed;
            keys.add(key);
          }
        }
        progress = parsed?.item?.progress;
      } catch { return closed; }
      if (!progress || typeof progress !== 'object' || Array.isArray(progress)) return closed;
      const seconds = Object.hasOwn(progress, 'seconds') ? progress.seconds : 0, watched = Object.hasOwn(progress, 'watched') ? progress.watched : false;
      if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds < 0 || seconds > 31622400
          || typeof watched !== 'boolean') return closed;
      return {schemaVersion: 1, stage, status, progress: {seconds, watched}};
    })().catch(() => value), new Promise(resolve => {timer = setTimeout(() => resolve(value), 1200);})]);
    clearTimeout(timer);
    const attached = await Promise.race([info.attach('playback-state', {body: JSON.stringify(facts), contentType: 'application/json'}).then(() => true),
      new Promise(resolve => {timer = setTimeout(() => resolve(false), 500);})]);
    return attached ? facts : value;
  } catch { /* A failed observation must preserve the recipe's original assertion. */ }
  finally {clearTimeout(timer);}
  return value;
}

// Watched titles intentionally require public playback intent; unavailable facts retain the automatic oracle.
export async function startWatchedPlayback(page, facts) {
  if (!facts || Object.keys(facts).sort().join(',') !== 'progress,schemaVersion,stage,status'
      || facts.schemaVersion !== 1 || facts.stage !== 'before-loading' || facts.status !== 200
      || !facts.progress || Object.keys(facts.progress).sort().join(',') !== 'seconds,watched'
      || typeof facts.progress.seconds !== 'number' || !Number.isFinite(facts.progress.seconds)
      || facts.progress.seconds < 0 || facts.progress.seconds > 31622400 || facts.progress.watched !== true) return;
  await page.locator('.media-stage').focus();
  await page.keyboard.press('Space');
}
