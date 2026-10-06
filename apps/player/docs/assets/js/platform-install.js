(() => {
  const builder = document.querySelector('[data-install-builder]');
  if (!builder) return;

  const field = name => builder.querySelector(`[data-install-${name}]`);
  const app = field('app');
  const media = field('media');
  const port = field('port');
  const portLabel = field('port-label');
  const subtitlesPort = field('subtitles-port');
  const subtitlesPortField = field('subtitles-port-field');
  const create = field('create');
  const error = field('error');
  const result = field('result');
  const preview = field('preview');
  const download = field('download');
  const copy = field('copy');
  const status = field('status');
  const specs = { player: { port: 38127 }, subtitles: { port: 38128 }, both: { port: 38127 } };
  let file = '';
  let fileUrl = '';
  let revision = 0;
  let clipboardNonce = 0;
  let active = null;
  const originalHrefs = [
    'https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose.yaml',
    'https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/subtitles/packaging/platform-compose.yaml',
    'https://raw.githubusercontent.com/Kinosail/kinosail/main/apps/player/packaging/platform-compose-both.yaml',
  ];
  const originals = originalHrefs.map(href =>
    [...document.querySelectorAll('a[href]')].find(link => link.getAttribute('href') === href));
  let fallback = null;
  if (originals.every(Boolean)) {
    fallback = document.createElement('p');
    fallback.className = 'install-hint';
    fallback.dataset.installFallback = '';
    fallback.hidden = true;
    fallback.textContent = 'Use an original Compose file: ';
    originals.forEach((original, index) => {
      const link = document.createElement('a');
      link.href = originalHrefs[index];
      link.textContent = original.textContent;
      fallback.append(link, index === originals.length - 1 ? '.' : ', ');
    });
    error.after(fallback);
  }

  function clear() {
    revision++;
    clipboardNonce++;
    const previous = active;
    active = null;
    if (previous) {
      clearTimeout(previous.timer);
      previous.cancel();
    }
    if (fallback) fallback.hidden = true;
    if (fileUrl) URL.revokeObjectURL(fileUrl);
    fileUrl = '';
    file = '';
    download.removeAttribute('href');
    preview.textContent = '';
    status.textContent = '';
    error.hidden = true;
    result.hidden = true;
    create.disabled = false;
    create.textContent = 'Make Compose file';
  }

  function fail(message, retry = false) {
    clear();
    error.textContent = message;
    error.hidden = false;
    if (retry) {
      create.textContent = 'Retry';
      if (fallback) fallback.hidden = false;
    }
  }

  function once(value, marker) {
    return value.split(marker).length === 2;
  }

  function validPort(value) {
    return /^[1-9][0-9]{0,4}$/.test(value) && Number(value) >= 1024 && Number(value) <= 65535;
  }

  async function makeFile() {
    clear();
    const choice = app.value;
    if (!Object.hasOwn(specs, choice)) return fail('Choose Player, Subtitles, or Both.');
    const path = media.value;
    if (path.length > 4096 || path !== path.trim() || path === '/' || !path.startsWith('/') || path.split('/').some(part => part === '.' || part === '..') || /[\u0000-\u001f\u007f-\u009f$]/u.test(path)) {
      return fail('Enter an absolute media path on the server. Do not use . or .. in the path.');
    }
    const hostPort = port.value.trim();
    const secondPort = subtitlesPort.value.trim();
    if (!validPort(hostPort) || (choice === 'both' && (!validPort(secondPort) || hostPort === secondPort))) {
      return fail(choice === 'both' ? 'Enter two different host ports from 1024 to 65535.' : 'Enter an unused host port from 1024 to 65535.');
    }
    const source = builder.dataset[`${choice}Template`];
    let address;
    try { address = new URL(source, location.href); }
    catch (_) { return fail('The install file is unavailable. Use the original Compose links below.', true); }
    if (address.origin !== location.origin || !address.pathname.endsWith(`/assets/install/${choice}.yaml`)) {
      return fail('The install file is unavailable. Use the original Compose links below.', true);
    }

    const current = revision;
    const attempt = { controller: new AbortController(), timer: 0, cancel: () => {} };
    active = attempt;
    const deadline = performance.now() + 15000;
    const owns = () => current === revision && active === attempt;
    const expired = () => attempt.controller.signal.aborted || performance.now() >= deadline;
    const stopped = new Promise((_, reject) => {
      attempt.cancel = () => {
        reject(new Error('Template stopped'));
        attempt.controller.abort();
      };
      attempt.timer = setTimeout(attempt.cancel, 15000);
    });
    create.disabled = true;
    create.textContent = 'Preparing file…';
    try {
      const loading = (async () => {
        const response = await fetch(source, { credentials: 'omit', signal: attempt.controller.signal });
        if (!owns()) return null;
        if (expired()) throw new Error('Template stopped');
        if (!response.ok) throw new Error('Template unavailable');
        return response.text();
      })();
      const template = await Promise.race([loading, stopped]);
      if (!owns()) return;
      if (expired()) throw new Error('Template stopped');
      const mediaMarker = 'source: "${KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"';
      const portMarker = `- "${specs[choice].port}:${specs[choice].port}"`;
      const both = choice === 'both';
      const secondPortMarker = '- "38128:38128"';
      if (template.length > 16384 || template.split(mediaMarker).length !== (both ? 3 : 2) || !once(template, portMarker) ||
          !once(template, `name: kinosail-${choice}`) ||
          (both ? !once(template, 'image: ghcr.io/kinosail/kinosail-player:latest') ||
            !once(template, 'image: ghcr.io/kinosail/kinosail-subtitles:latest') ||
            !once(template, secondPortMarker) ||
            !template.includes('read_only: true\n        bind:') ||
            !template.includes('read_only: false\n        bind:') :
            !once(template, `image: ghcr.io/kinosail/kinosail-${choice}:latest`) ||
            !template.includes(`read_only: ${choice === 'player' ? 'true' : 'false'}\n        bind:`)) ||
          template.split('user: "10001:10001"').length !== (both ? 3 : 2) ||
          template.split('target: /media').length !== (both ? 3 : 2) ||
          template.split('create_host_path: false').length !== (both ? 3 : 2)) {
        throw new Error('Unexpected template');
      }
      const nextFile = template.replaceAll(mediaMarker, `source: ${JSON.stringify(path)}`)
        .replace(portMarker, `- "${hostPort}:${specs[choice].port}"`)
        .replace(secondPortMarker, both ? `- "${secondPort}:38128"` : secondPortMarker);
      if (!owns()) return;
      if (expired()) throw new Error('Template stopped');
      const nextUrl = URL.createObjectURL(new Blob([nextFile], { type: 'application/yaml;charset=utf-8' }));
      if (!owns() || expired()) {
        URL.revokeObjectURL(nextUrl);
        if (owns()) throw new Error('Template stopped');
        return;
      }
      file = nextFile;
      fileUrl = nextUrl;
      download.href = fileUrl;
      download.download = `kinosail-${choice}-compose.yaml`;
      preview.textContent = file;
      result.hidden = false;
      status.textContent = 'File ready. Download it or copy it into your app manager.';
    } catch (_) {
      if (owns()) fail('Could not prepare the file. Retry or use the original Compose links below.', true);
    } finally {
      clearTimeout(attempt.timer);
      if (owns()) {
        active = null;
        create.disabled = false;
        create.textContent = 'Make Compose file';
      }
    }
  }

  app.addEventListener('change', () => {
    port.value = specs[app.value]?.port ?? '';
    subtitlesPort.value = '38128';
    subtitlesPortField.hidden = app.value !== 'both';
    portLabel.textContent = app.value === 'both' ? 'Player HTTPS port on the server' : 'HTTPS port on the server';
    clear();
  });
  media.addEventListener('input', clear);
  port.addEventListener('input', clear);
  subtitlesPort.addEventListener('input', clear);
  create.addEventListener('click', makeFile);
  copy.addEventListener('click', async () => {
    if (!file) return;
    const current = revision, copiedFile = file, nonce = ++clipboardNonce;
    const owns = () => current === revision && file === copiedFile && nonce === clipboardNonce;
    try {
      await navigator.clipboard.writeText(copiedFile);
      if (owns()) status.textContent = 'Compose file copied.';
    } catch (_) {
      if (owns()) status.textContent = 'Copy failed. Download the file or open Review the file to copy its text.';
    }
  });
  window.addEventListener('pagehide', clear);
  builder.hidden = false;
})();
