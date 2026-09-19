// Camera frames stay in this browser; only a validated code reaches confirmation.
(() => {
  const start = document.querySelector('[data-qr-start]');
  if (!start) return;
  const camera = document.querySelector('[data-qr-camera]');
  const video = document.querySelector('[data-qr-video]');
  const stop = document.querySelector('[data-qr-stop]');
  const status = document.querySelector('[data-qr-status]');
  const canvas = document.createElement('canvas');
  const context = canvas.getContext('2d', { willReadFrequently: true });
  let generation = 0;
  let stream;
  let timer;
  let decoder;

  function message(text) {
    status.hidden = false;
    status.textContent = text;
  }

  function close(restoreFocus = false) {
    generation++;
    clearTimeout(timer);
    stream?.getTracks().forEach(track => track.stop());
    stream = null;
    video.srcObject = null;
    camera.hidden = true;
    start.disabled = false;
    if (restoreFocus) start.focus();
  }

  function readCode(raw) {
    if (typeof raw !== 'string' || raw.length > 2048) return null;
    if (/^[0-9]{6}$/.test(raw)) return raw;
    // Accept only the exact same-server links emitted by Player clients.
    if (!/^https?:\/\//.test(raw) || /[\s\\]/.test(raw)) return null;
    try {
      const url = new URL(raw);
      if (url.origin !== location.origin || url.username || url.password || url.hash) return null;
      if (!['/connect', '/quick-connect'].includes(url.pathname)) return null;
      if (!/^\?code=[0-9]{6}$/.test(url.search)) return null;
      return url.searchParams.get('code');
    } catch { return null; }
  }

  function loadDecoder() {
    if (!decoder) decoder = new Promise((resolve, reject) => {
      const script = document.createElement('script');
      script.src = '/static/qr-decoder.js?v=1.4.0';
      script.onload = () => typeof window.jsQR === 'function' ? resolve(window.jsQR) : reject();
      script.onerror = () => { script.remove(); reject(); };
      document.head.append(script);
    }).catch(error => { decoder = null; throw error; });
    return decoder;
  }

  function scan(token, decode) {
    if (token !== generation) return;
    try {
      if (video.readyState >= 2 && video.videoWidth && video.videoHeight) {
        const scale = Math.min(1, 640 / Math.max(video.videoWidth, video.videoHeight));
        canvas.width = Math.max(1, Math.round(video.videoWidth * scale));
        canvas.height = Math.max(1, Math.round(video.videoHeight * scale));
        context.drawImage(video, 0, 0, canvas.width, canvas.height);
        const pixels = context.getImageData(0, 0, canvas.width, canvas.height);
        const result = decode(pixels.data, pixels.width, pixels.height);
        if (result) {
          const code = readCode(result.data);
          if (code) {
            close();
            location.assign('/quick-connect?code=' + code);
            return;
          }
          message('Scan the QR code shown by a TV or app connected to this Server.');
        }
      }
      timer = setTimeout(() => scan(token, decode), 200);
    } catch {
      close(true);
      message('Could not read the camera. Try again or enter the six-digit code.');
    }
  }

  start.hidden = false;
  start.addEventListener('click', async () => {
    if (!window.isSecureContext || !navigator.mediaDevices?.getUserMedia || !context) {
      message('Camera scanning is unavailable here. Open Player over HTTPS, or enter the six-digit code.');
      return;
    }
    close();
    const token = generation;
    start.disabled = true;
    camera.hidden = false;
    stop.focus();
    message('Allow camera access, then point your camera at the QR code on your TV or app.');
    try {
      const decode = await loadDecoder();
      if (token !== generation) return;
      const acquired = await navigator.mediaDevices.getUserMedia({
        audio: false, video: { facingMode: { ideal: 'environment' } },
      });
      if (token !== generation) {
        acquired.getTracks().forEach(track => track.stop());
        return;
      }
      stream = acquired;
      video.srcObject = stream;
      await video.play();
      if (token !== generation) return;
      scan(token, decode);
    } catch (error) {
      if (token !== generation) return;
      close(true);
      message(error?.name === 'NotAllowedError'
        ? 'Camera access was denied. Allow it in your browser settings, or enter the six-digit code.'
        : 'Could not open the scanner. Try again or enter the six-digit code.');
    }
  });
  stop.addEventListener('click', () => {
    close(true);
    message('Scanning stopped. You can enter the six-digit code instead.');
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && !camera.hidden) stop.click();
  });
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) { close(); status.hidden = true; }
  });
  window.addEventListener('pagehide', () => close());
  start.form?.addEventListener('submit', () => close());
})();
