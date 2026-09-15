(() => {
  const dialog = document.querySelector('[data-tv-picker]');
  const endpoint = () => player.dataset.castApi;
  if (!dialog || !endpoint()) return;
  const button = (selector) => dialog.querySelector(selector);
  const message = (text) => { button('[data-tv-status]').textContent = text; };
  const native = button('[data-cast]');
  const nativeAvailable = () => { button('[data-tv-native-unavailable]').hidden = native && !native.hidden; };
  new MutationObserver(nativeAvailable).observe(native, { attributes: true, attributeFilter: ['hidden'] });
  nativeAvailable();
  document.querySelectorAll('[data-tv-open]').forEach((open) => open.addEventListener('click', () => { open.focus({preventScroll: true}); dialog.showModal(); }));
  button('[data-tv-close]').addEventListener('click', () => dialog.close());
  let busy = false, session, controller, position = 0, timer, closing = false, castContext;
  const validID = (value) => typeof value === 'string' && /^[a-f0-9]{32}$/.test(value);
  const validPosition = (value) => Number.isFinite(value) && value >= 0 && value <= 31536000;
  const request = async (path, method = 'GET', body) => {
    const response = await fetch(path, { method, headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-Kinosail-CSRF': csrf } : {}) }, ...(body === undefined ? {} : { body: JSON.stringify(body) }) });
    if (!response.ok) throw new Error('Kinosail could not complete TV playback. Check the connection and this title’s compatibility.');
    if (response.status === 204) return;
    const text = await response.text();
    if (text.length > 65536) throw new Error('TV response is too large.');
    return JSON.parse(text);
  };
  const run = async (action) => {
    if (busy) return;
    busy = true;
    dialog.setAttribute('aria-busy', 'true');
    try { await action(); }
    catch (error) { message(error instanceof Error ? error.message : 'TV playback is unavailable.'); }
    finally { busy = false; dialog.removeAttribute('aria-busy'); }
  };
  const revoke = async (id) => {
    const response = await fetch(`/api/v1/cast/sessions/${id}`, { method: 'DELETE', headers: csrf ? { 'X-Kinosail-CSRF': csrf } : {}, keepalive: true });
    if (!response.ok && response.status !== 404) throw new Error('Could not revoke TV access. Reconnect to the Server and try again.');
  };
  const parseSession = (value) => {
    if (!value || !validID(value.id) || !validPosition(value.position) || !validPosition(value.duration) || typeof value.url !== 'string' || value.url.length > 4096 || typeof value.title !== 'string' || value.title.length > 1024) throw new Error('Invalid TV session.');
    const url = new URL(value.url);
    if (url.origin !== location.origin || url.username || url.password || url.hash || ![`/cast/${value.id}/media`, `/cast/${value.id}/hls/index.m3u8`].includes(url.pathname) || [...url.searchParams.keys()].length !== 1 || !/^[a-f0-9]{64}$/.test(url.searchParams.get('ticket') || '') || !/^(audio\/|video\/|application\/vnd\.apple\.mpegurl$)/.test(value.contentType)) throw new Error('Invalid TV media address.');
    if (!Array.isArray(value.tracks) || value.tracks.length > 64) throw new Error('Invalid TV subtitles.');
    value.tracks.forEach((track, index) => {
      const source = new URL(track.url);
      if (track.id !== index + 1 || source.origin !== url.origin || source.pathname !== `/cast/${value.id}/subtitles/${track.id}` || source.search !== url.search || source.hash || source.username || source.password || typeof track.label !== 'string' || track.label.length > 512 || typeof track.language !== 'string' || track.language.length > 32) throw new Error('Invalid TV subtitles.');
    });
    return value;
  };
  const saveTV = async (watched = false) => {
    if (!session) return;
    const id = endpoint().split('/').at(-2);
    await request(`/api/v1/items/${encodeURIComponent(id)}/progress`, 'PUT', { seconds: position, session: playbackSession, revision: ++progressRevision, playbackToken: '', ...(watched ? { watched: true } : {}) });
  };
  const poll = async () => {
    if (!session || closing) return;
    try {
      const observed = session;
      const state = await controller.status();
      if (session !== observed || closing) return;
      if (!state || !validPosition(state.position) || !['playing', 'paused', 'buffering', 'stopped'].includes(state.state) || session.duration > 0 && state.position > session.duration + 2) throw new Error('Invalid TV state.');
      position = state.position;
      button('[data-tv-position]').textContent = `${Math.floor(position / 60)}:${String(Math.floor(position % 60)).padStart(2, '0')} · ${state.state}`;
      if (state.state === 'playing' || state.state === 'paused') await saveTV();
      else if (state.state === 'stopped' && session.duration > 0 && position >= session.duration - 2) await saveTV(true);
    } catch { message('The TV is not responding. Check its connection or stop casting.'); }
    finally { if (session && !closing) timer = setTimeout(poll, 3000); }
  };
  const adopt = async (media, remoteController, name) => {
    player.pause();
    await save(false);
    session = media; controller = remoteController; position = media.position;
    player.dataset.castActive = 'true'; document.body.dataset.tvActive = 'true';
    button('[data-tv-controls]').hidden = false;
    button('[data-tv-target]').textContent = `Playing on ${name}`;
    message('Connected. Use these controls for the TV.');
    void poll();
  };
  player.addEventListener('play', () => { if (session) { player.pause(); dialog.showModal(); } });
  const begin = async (protocol, deviceId) => {
    if (session) throw new Error('Stop the current TV session before choosing another.');
    return parseSession(await request(endpoint(), 'POST', { protocol, ...(deviceId ? { deviceId } : {}), position: Number.isFinite(player.currentTime) ? player.currentTime : 0, playbackToken: player.dataset.playbackToken || '' }));
  };
  button('[data-tv-scan]').addEventListener('click', () => run(async () => {
    const result = await request('/api/v1/cast/devices/scan', 'POST', {});
    if (!Array.isArray(result.devices) || result.devices.length > 64) throw new Error('Invalid TV discovery response.');
    const seen = new Set();
    result.devices.forEach((device) => { if (!validID(device.id) || seen.has(device.id) || device.protocol !== 'dlna' || typeof device.name !== 'string' || !device.name.trim() || device.name.length > 128 || /[\x00-\x1f\x7f]/.test(device.name)) throw new Error('Invalid TV discovery response.'); seen.add(device.id); });
    const list = button('[data-tv-devices]'); list.replaceChildren();
    result.devices.forEach((device) => {
      const choice = document.createElement('button'); choice.type = 'button'; choice.className = 'quiet'; choice.textContent = `Play on ${device.name} · DLNA`;
      choice.addEventListener('click', () => run(async () => {
        const media = await begin('dlna', device.id);
        try { await adopt(media, { status: () => request(`/api/v1/cast/sessions/${media.id}`), command: (command) => request(`/api/v1/cast/sessions/${media.id}/commands`, 'POST', command) }, device.name); } catch (error) { await request(`/api/v1/cast/sessions/${media.id}/commands`, 'POST', { action: 'stop' }).catch(() => {}); await revoke(media.id); throw error; }
      })); list.append(choice);
    });
    message(result.devices.length ? 'Choose a TV.' : 'No DLNA TVs found. Enable media renderer mode on your TV and search again.');
  }));
  button('[data-tv-google]').addEventListener('click', () => run(async () => {
    if (!castContext) {
      if (!window.isSecureContext) throw new Error('Open Kinosail over HTTPS to use Google Cast in the browser.');
      await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('Google Cast did not load. Try a supported Chrome browser.')), 20000);
        window.__onGCastApiAvailable = (available) => { clearTimeout(timeout); available ? resolve() : reject(new Error('Google Cast is unavailable in this browser.')); };
        const script = document.createElement('script'); script.src = 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1'; script.onerror = () => { clearTimeout(timeout); reject(new Error('Google Cast could not load.')); }; document.head.append(script);
      });
      castContext = cast.framework.CastContext.getInstance();
      castContext.setOptions({ receiverApplicationId: chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID, autoJoinPolicy: chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED });
      button('[data-tv-google]').textContent = 'Choose Google Cast TV';
      message('Google Cast is ready. Choose your TV to start this title.');
      return;
    }
    if (session) throw new Error('Stop the current TV session before choosing another.');
    await castContext.requestSession();
    const selected = castContext.getCurrentSession();
    if (!selected) return;
    const media = await begin('google-cast');
    try {
      const info = new chrome.cast.media.MediaInfo(media.url, media.contentType);
      info.metadata = new chrome.cast.media.GenericMediaMetadata(); info.metadata.title = media.title;
      info.duration = media.duration;
      info.tracks = media.tracks.map((track) => { const value = new chrome.cast.media.Track(track.id, chrome.cast.media.TrackType.TEXT); value.trackContentId = track.url; value.trackContentType = 'text/vtt'; value.name = track.label; value.language = track.language || 'und'; value.subtype = chrome.cast.media.TextTrackType.SUBTITLES; return value; });
      if (media.contentType === 'application/vnd.apple.mpegurl') { info.hlsSegmentFormat = chrome.cast.media.HlsSegmentFormat.FMP4; info.hlsVideoSegmentFormat = chrome.cast.media.HlsVideoSegmentFormat.FMP4; }
      const load = new chrome.cast.media.LoadRequest(info); load.currentTime = media.position; load.autoplay = true; load.activeTrackIds = media.tracks.filter((track) => track.default).map((track) => track.id);
      await selected.loadMedia(load);
      const get = () => { const current = selected.getMediaSession(); if (castContext.getCurrentSession() !== selected || !current || current.media.contentId !== media.url) throw new Error('The TV is playing a different title.'); return current; };
      await adopt(media, { status: async () => { const current = get(); if (current.idleReason === chrome.cast.media.IdleReason.ERROR) throw new Error('The TV could not play this title.'); return { state: current.playerState === 'IDLE' ? 'stopped' : current.playerState === 'BUFFERING' ? 'buffering' : current.playerState.toLowerCase(), position: current.getEstimatedTime() }; }, command: (command) => new Promise((resolve, reject) => { const current = get(); if (command.action === 'seek') { const seek = new chrome.cast.media.SeekRequest(); seek.currentTime = command.position; current.seek(seek, resolve, reject); } else current[command.action](null, resolve, reject); }) }, selected.getCastDevice().friendlyName || 'Google Cast TV');
    } catch (error) { const loaded = selected.getMediaSession(); if (loaded?.media?.contentId === media.url) await new Promise((resolve) => loaded.stop(null, resolve, resolve)); await revoke(media.id); throw error; }
  }));
  dialog.querySelectorAll('[data-tv-command]').forEach((control) => control.addEventListener('click', () => run(async () => {
    if (!session) return;
    const action = control.dataset.tvCommand;
    await controller.command(action === 'back' || action === 'forward' ? { action: 'seek', position: Math.max(0, Math.min(session.duration, position + (action === 'back' ? -10 : 10))) } : { action });
  })));
  button('[data-tv-stop]').addEventListener('click', () => run(async () => {
    if (!session) return;
    closing = true; clearTimeout(timer);
    try { await controller.command({ action: 'stop' }); } catch { /* Revocation remains available when the TV is offline. */ }
    try { await revoke(session.id); }
    catch (error) { closing = false; void poll(); throw error; }
    session = undefined; controller = undefined; closing = false;
    delete player.dataset.castActive; delete document.body.dataset.tvActive;
    button('[data-tv-controls]').hidden = true; message('Casting stopped. Reload the player to resume here from the saved TV position.');
    const resume = document.createElement('button'); resume.type = 'button'; resume.className = 'quiet'; resume.textContent = 'Resume on this device'; resume.addEventListener('click', () => location.reload()); button('[data-tv-status]').append(resume);
  }));
  addEventListener('pagehide', () => { closing = true; clearTimeout(timer); if (session) void revoke(session.id).catch(() => {}); }, { once: true });
})();
