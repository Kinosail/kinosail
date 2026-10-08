// One routed quota fixture needs real OPFS in an owned persistent WebKit profile.
export async function withOPFSPage(input, use) {
  const allowed = ['page', 'browserName', 'webkit', 'baseURL', 'viewport', 'userAgent', 'deviceScaleFactor', 'isMobile', 'hasTouch'];
  if (!input || typeof input !== 'object' || Array.isArray(input) || Object.keys(input).some(key => !allowed.includes(key))
      || allowed.some(key => !Object.hasOwn(input, key)) || !['chromium', 'firefox', 'webkit'].includes(input.browserName)
      || typeof input.baseURL !== 'string' || !input.baseURL || input.baseURL.length > 2048) throw new Error('invalid OPFS fixture options');
  let origin;
  try {origin = new URL(input.baseURL);} catch {throw new Error('invalid OPFS fixture origin');}
  const viewport = input.viewport;
  if (!['http:', 'https:'].includes(origin.protocol) || !['localhost', '127.0.0.1'].includes(origin.hostname)
      || origin.username || origin.password || origin.port === '0' || ![origin.origin, origin.origin + '/'].includes(input.baseURL)
      || !viewport || Object.keys(viewport).sort().join(',') !== 'height,width'
      || ![viewport.width, viewport.height].every(value => Number.isInteger(value) && value >= 1 && value <= 8192)
      || typeof input.userAgent !== 'string' || !input.userAgent || input.userAgent.length > 1024 || /[\x00-\x1f\x7f]/.test(input.userAgent)
      || !Number.isFinite(input.deviceScaleFactor) || input.deviceScaleFactor < .1 || input.deviceScaleFactor > 10
      || typeof input.isMobile !== 'boolean' || typeof input.hasTouch !== 'boolean') throw new Error('invalid OPFS fixture options');
  if (input.browserName !== 'webkit') return use(input.page);
  const context = await input.webkit.launchPersistentContext('', {baseURL: origin.origin, viewport: {...viewport},
    userAgent: input.userAgent, deviceScaleFactor: input.deviceScaleFactor, isMobile: input.isMobile, hasTouch: input.hasTouch,
    serviceWorkers: 'block', ignoreHTTPSErrors: false, timeout: 10000});
  let failure;
  try {await use(context.pages()[0] ?? await context.newPage());}
  catch (error) {failure = error; throw error;}
  finally {try {await context.close();} catch (error) {if (!failure) throw error;}}
}

// Runs in the real page. Missing API alone is a capability disposition.
export async function writeOrphanedOPFS(id) {
  if (typeof id !== 'string' || !/^[a-f0-9]{16}$/.test(id)) throw new Error('invalid OPFS fixture identity');
  const scope = async jobID => {
    if (typeof FileSystemFileHandle === 'undefined' || !FileSystemFileHandle.prototype.createSyncAccessHandle)
      return self.postMessage({supported: false});
    try {
      const file = await (await navigator.storage.getDirectory()).getFileHandle(jobID, {create: true});
      const writer = await file.createSyncAccessHandle();
      try {writer.write(new Uint8Array(16).fill(1)); writer.flush();}
      finally {writer.close();}
      self.postMessage({supported: true});
    } catch (error) {
      const names = ['UnknownError', 'QuotaExceededError', 'SecurityError', 'NotAllowedError', 'NotFoundError', 'InvalidStateError', 'TypeError'];
      self.postMessage({error: names.includes(error?.name) ? error.name : 'OPFSWriteError'});
    }
  };
  const url = URL.createObjectURL(new Blob([`(${scope.toString()})(${JSON.stringify(id)})`], {type: 'text/javascript'}));
  let worker, timer;
  try {
    worker = new Worker(url);
    return await new Promise((resolve, reject) => {
      timer = setTimeout(() => reject(new Error('OPFS worker deadline')), 10000);
      worker.onmessage = ({data}) => {
        if (!data || typeof data !== 'object' || Array.isArray(data) || Object.keys(data).length !== 1)
          return reject(new Error('invalid OPFS worker result'));
        if (Object.hasOwn(data, 'supported') && typeof data.supported === 'boolean') return resolve(data.supported);
        if (Object.hasOwn(data, 'error') && ['UnknownError', 'QuotaExceededError', 'SecurityError', 'NotAllowedError',
          'NotFoundError', 'InvalidStateError', 'TypeError', 'OPFSWriteError'].includes(data.error))
          return reject(Object.assign(new Error('OPFS orphan write failed'), {name: data.error}));
        reject(new Error('invalid OPFS worker result'));
      };
      worker.onerror = worker.onmessageerror = () => reject(new Error('OPFS worker failed'));
    });
  } finally {
    clearTimeout(timer);
    if (worker) {worker.onmessage = worker.onerror = worker.onmessageerror = null; worker.terminate();}
    URL.revokeObjectURL(url);
  }
}
