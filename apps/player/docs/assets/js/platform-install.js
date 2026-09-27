(() => {
  const builder = document.querySelector('[data-install-builder]');
  if (!builder) return;

  const field = name => builder.querySelector(`[data-install-${name}]`);
  const app = field('app');
  const media = field('media');
  const port = field('port');
  const create = field('create');
  const error = field('error');
  const result = field('result');
  const preview = field('preview');
  const download = field('download');
  const copy = field('copy');
  const status = field('status');
  const specs = { player: { port: 38127 }, subtitles: { port: 38128 } };
  let file = '';
  let fileUrl = '';
  let revision = 0;

  function clear() {
    revision++;
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

  function fail(message) {
    clear();
    error.textContent = message;
    error.hidden = false;
  }

  function once(value, marker) {
    return value.split(marker).length === 2;
  }

  async function makeFile() {
    clear();
    const choice = app.value;
    if (!Object.hasOwn(specs, choice)) return fail('Choose Player or Subtitles.');
    const path = media.value;
    if (path.length > 4096 || path !== path.trim() || path === '/' || !path.startsWith('/') || path.split('/').some(part => part === '.' || part === '..') || /[\u0000-\u001f\u007f-\u009f$]/u.test(path)) {
      return fail('Enter an absolute media path on the server. Do not use . or .. in the path.');
    }
    const hostPort = port.value.trim();
    if (!/^[1-9][0-9]{0,4}$/.test(hostPort) || Number(hostPort) < 1024 || Number(hostPort) > 65535) {
      return fail('Enter an unused host port from 1024 to 65535.');
    }
    const source = builder.dataset[`${choice}Template`];
    const address = new URL(source, location.href);
    if (address.origin !== location.origin || !address.pathname.endsWith(`/assets/install/${choice}.yaml`)) {
      return fail('The install file is unavailable. Use the original Compose links below.');
    }

    const current = revision;
    create.disabled = true;
    create.textContent = 'Preparing file…';
    try {
      const response = await fetch(source, { credentials: 'omit' });
      if (current !== revision) return;
      if (!response.ok) throw new Error('Template unavailable');
      const template = await response.text();
      if (current !== revision) return;
      const mediaMarker = 'source: "${KINOSAIL_MEDIA_PATH:?Set an existing absolute media path}"';
      const portMarker = `- "${specs[choice].port}:${specs[choice].port}"`;
      if (template.length > 16384 || !once(template, mediaMarker) || !once(template, portMarker) ||
          !once(template, `name: kinosail-${choice}`) ||
          !once(template, `image: ghcr.io/kinosail/kinosail-${choice}:latest`) ||
          !once(template, 'user: "10001:10001"') || !once(template, 'target: /media') ||
          !once(template, 'create_host_path: false') ||
          !template.includes(`read_only: ${choice === 'player' ? 'true' : 'false'}\n        bind:`)) {
        throw new Error('Unexpected template');
      }
      file = template.replace(mediaMarker, `source: ${JSON.stringify(path)}`)
        .replace(portMarker, `- "${hostPort}:${specs[choice].port}"`);
      fileUrl = URL.createObjectURL(new Blob([file], { type: 'application/yaml;charset=utf-8' }));
      download.href = fileUrl;
      download.download = `kinosail-${choice}-compose.yaml`;
      preview.textContent = file;
      result.hidden = false;
      status.textContent = 'File ready. Download it or copy it into your app manager.';
    } catch (_) {
      if (current === revision) fail('Could not prepare the file. Use the original Compose links below.');
    } finally {
      if (current === revision) {
        create.disabled = false;
        create.textContent = 'Make Compose file';
      }
    }
  }

  app.addEventListener('change', () => {
    port.value = specs[app.value]?.port ?? '';
    clear();
  });
  media.addEventListener('input', clear);
  port.addEventListener('input', clear);
  create.addEventListener('click', makeFile);
  copy.addEventListener('click', async () => {
    if (!file) return;
    try {
      await navigator.clipboard.writeText(file);
      status.textContent = 'Compose file copied.';
    } catch (_) {
      status.textContent = 'Copy failed. Download the file or open Review the file to copy its text.';
    }
  });
  window.addEventListener('pagehide', () => { if (fileUrl) URL.revokeObjectURL(fileUrl); });
  builder.hidden = false;
})();
