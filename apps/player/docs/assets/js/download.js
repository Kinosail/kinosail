(() => {
  const root = document.querySelector('[data-download]');
  if (!root) return;

  const RELEASE_API = 'https://api.github.com/repos/Kinosail/kinosail/releases/latest';
  const RELEASES = 'https://github.com/Kinosail/kinosail/releases';
  const TAG = /^player-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;
  const FILE = /^Kinosail-(Windows|macOS|Linux)(?:-(x64|arm64))?\.(msi|dmg|pkg|deb|rpm)$/;
  const SYSTEM_NAMES = { windows: 'Windows', macos: 'macOS', linux: 'Linux' };
  const field = name => root.querySelector(`[data-download-${name}]`);
  const detected = field('detected');
  const primary = field('primary');
  const button = field('button');
  const meta = field('meta');
  const note = field('note');

  const show = (element, text) => {
    element.textContent = text;
    element.hidden = false;
  };

  const detectSystem = () => {
    const platform = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
    const agent = navigator.userAgent || '';
    if (/android|iphone|ipad|ipod/i.test(agent) || (/Mac/.test(platform) && navigator.maxTouchPoints > 1)) return 'mobile';
    if (/win/i.test(platform) || /Windows/.test(agent)) return 'windows';
    if (/mac/i.test(platform) || /Mac OS X/.test(agent)) return 'macos';
    if (/linux|x11|cros/i.test(platform) || /Linux/.test(agent)) return 'linux';
    return 'other';
  };

  const detectArch = async () => {
    if (/aarch64|arm64|armv8/i.test(navigator.userAgent || '')) return 'arm64';
    const data = navigator.userAgentData;
    if (data && typeof data.getHighEntropyValues === 'function') {
      try {
        const values = await data.getHighEntropyValues(['architecture', 'bitness']);
        if (values.architecture === 'arm') return 'arm64';
        if (values.architecture === 'x86') return 'x64';
      } catch (error) {
        return guessArch();
      }
    }
    return guessArch();
  };

  const guessArch = () => /aarch64|arm64|armv8/i.test(navigator.userAgent || navigator.platform || '') ? 'arm64' : 'x64';

  const fetchRelease = async () => {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 8000);
    try {
      const response = await fetch(RELEASE_API, { headers: { Accept: 'application/vnd.github+json' }, signal: controller.signal });
      if (!response.ok) return null;
      const release = await response.json();
      if (!release || typeof release.tag_name !== 'string' || !TAG.test(release.tag_name) || !Array.isArray(release.assets)) return null;
      const files = new Map();
      for (const asset of release.assets.slice(0, 100)) {
        if (asset && typeof asset.name === 'string' && FILE.test(asset.name)
            && typeof asset.browser_download_url === 'string'
            && asset.browser_download_url.startsWith(`${RELEASES}/download/${release.tag_name}/`)) {
          files.set(asset.name, { url: asset.browser_download_url, size: Number(asset.size) || 0 });
        }
      }
      return { version: release.tag_name.slice('player-'.length), url: release.html_url, published: release.published_at, files };
    } catch (error) {
      return null;
    } finally {
      clearTimeout(timer);
    }
  };

  const candidates = (system, arch) => {
    if (system === 'windows') return [`Kinosail-Windows-${arch}.msi`, 'Kinosail-Windows-x64.msi'];
    if (system === 'macos') return ['Kinosail-macOS.dmg', 'Kinosail-macOS.pkg'];
    if (system === 'linux') return [`Kinosail-Linux-${arch}.deb`, `Kinosail-Linux-${arch}.rpm`];
    return [];
  };

  const megabytes = size => size > 0 ? `${Math.round(size / 1048576)} MB` : '';
  const released = value => {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '' : `released ${date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })}`;
  };

  const versionLinks = release => {
    for (const link of document.querySelectorAll('a[href^="https://github.com/Kinosail/kinosail/releases/latest/download/"]')) {
      const name = link.getAttribute('href').split('/').pop();
      const file = release.files.get(name);
      if (file) link.href = file.url;
    }
  };

  const render = async () => {
    const system = detectSystem();
    if (system === 'mobile') {
      show(detected, 'Kinosail Server runs on a computer that stays on at home. Open this page on that computer to download it. Then watch on this device in your browser.');
      return;
    }
    if (system === 'other') {
      show(detected, 'We could not tell which computer you use. Choose it below.');
      return;
    }
    const arch = await detectArch();
    const label = `${SYSTEM_NAMES[system]}${system === 'macos' ? '' : arch === 'arm64' ? ' (ARM)' : ' (Intel or AMD)'}`;
    show(detected, `Your computer: ${label}.`);
    const release = await fetchRelease();
    if (!release) {
      show(note, 'We could not check the newest version right now. Use the links below. They always point to the newest release.');
      return;
    }
    versionLinks(release);
    const name = candidates(system, arch).find(candidate => release.files.has(candidate));
    if (!name) {
      show(note, `The ${SYSTEM_NAMES[system]} installer is coming soon. Until then, use Docker Desktop with the Docker quickstart.`);
      return;
    }
    const file = release.files.get(name);
    button.href = file.url;
    button.textContent = `Download Kinosail ${release.version} for ${SYSTEM_NAMES[system]}`;
    primary.hidden = false;
    show(meta, [name, megabytes(file.size), released(release.published)].filter(Boolean).join(' · '));
    if (system === 'linux') show(note, 'On Fedora, RHEL, or openSUSE, use the .rpm file below. The one-step install command picks the right file for you.');
  };

  render();
})();
