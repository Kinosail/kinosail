(() => {
  const root = document.querySelector('[data-download]');
  if (!root) return;

  const field = name => root.querySelector(`[data-download-${name}]`);
  const title = field('title');
  const text = field('text');
  const primary = field('primary');
  const button = field('button');
  const small = field('small');
  const command = field('command');
  const copy = field('copy');
  const fileLink = id => document.querySelector(`a[data-download-file="${id}"]`);

  const detectSystem = () => {
    const platform = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
    const agent = navigator.userAgent || '';
    if ((navigator.userAgentData && navigator.userAgentData.mobile) || /android|iphone|ipad|ipod/i.test(agent) || (/Mac/.test(platform) && navigator.maxTouchPoints > 1)) return 'mobile';
    if (/win/i.test(platform) || /Windows/.test(agent)) return 'windows';
    if (/mac/i.test(platform) || /Mac OS X/.test(agent)) return 'macos';
    if (/cros/i.test(platform) || /CrOS/.test(agent)) return 'other';
    if (/linux|x11/i.test(platform) || /Linux/.test(agent)) return 'linux';
    return 'other';
  };

  const detectArch = async () => {
    if (/aarch64|arm64|armv8/i.test(navigator.userAgent || '')) return 'arm64';
    const data = navigator.userAgentData;
    if (data && typeof data.getHighEntropyValues === 'function') {
      try {
        const values = await data.getHighEntropyValues(['architecture']);
        if (values.architecture === 'arm') return 'arm64';
      } catch (error) {
        return 'x64';
      }
    }
    return 'x64';
  };

  const usesRpm = () => /Fedora|Red Hat|CentOS|Rocky|AlmaLinux|openSUSE|SUSE/i.test(navigator.userAgent || '');

  const say = (heading, body) => {
    title.textContent = heading;
    text.textContent = body;
  };

  const offer = (label, link, note) => {
    button.textContent = label;
    button.href = link.href;
    primary.hidden = false;
    small.textContent = note;
    small.hidden = false;
  };

  if (copy) {
    copy.addEventListener('click', async () => {
      try {
        await navigator.clipboard.writeText(command.querySelector('code').textContent);
        copy.textContent = 'Copied';
      } catch (error) {
        copy.textContent = 'Select the text to copy';
      }
    });
  }

  const render = async () => {
    const system = detectSystem();
    if (system === 'mobile') {
      say('Install Kinosail on a computer', 'Kinosail runs on a computer that stays on at home. Open this page on that computer to download it. Then watch on this device.');
      return;
    }
    if (system === 'windows') {
      say('Kinosail for Windows is coming soon', 'Until then, you can run Kinosail with Docker Desktop.');
      return;
    }
    if (system === 'macos') {
      say('Kinosail for Mac is coming soon', 'Until then, you can run Kinosail with Docker Desktop.');
      return;
    }
    if (system !== 'linux') {
      say('Choose your computer', 'We could not tell which computer you use. Pick it from the list below.');
      return;
    }
    const arch = await detectArch();
    const rpm = usesRpm();
    const link = fileLink(`linux-${arch}-${rpm ? 'rpm' : 'deb'}`);
    if (!link) return;
    const chip = arch === 'arm64' ? 'ARM' : 'Intel or AMD';
    const family = rpm ? 'Fedora, RHEL, and openSUSE' : 'Ubuntu, Debian, Mint, and Raspberry Pi OS';
    say('Kinosail for Linux', 'Free. Runs in the background and starts by itself.');
    offer('Download for Linux', link, `For ${family} on ${chip}. Different Linux? See all downloads below.`);
    command.hidden = false;
  };

  render();
})();
