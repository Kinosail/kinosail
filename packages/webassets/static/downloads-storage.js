// One worker owns the file handle for a transfer. Flush each verified block
// before its IndexedDB metadata can make it visible to offline playback.
function offlineStorageWorkerScope() {
  let handle, size;
  const operations = new Set(["read", "write", "truncate", "close"]);
  self.onmessage = async ({data: message}) => {
    const {id, type, value} = message;
    try {
      let result;
      if (type === "open") {
        if (handle || !/^[0-9a-f]{16}$/.test(value?.jobID || "") || !Number.isSafeInteger(value.size) || value.size <= 0 || value.size > 128 * 1024 ** 3) throw new Error("Invalid offline file");
        if (typeof FileSystemFileHandle === "undefined" || !FileSystemFileHandle.prototype.createSyncAccessHandle) { self.postMessage({id, value: false}); return; }
        size = value.size;
        const file = await (await navigator.storage.getDirectory()).getFileHandle(value.jobID, {create: true});
        handle = await file.createSyncAccessHandle();
        result = true;
      } else {
        if (!handle || !operations.has(type)) throw new Error("Offline writer unavailable");
        if (type === "close") { handle.flush(); handle.close(); handle = undefined; }
        else if (type === "truncate") { handle.truncate(size); handle.flush(); }
        else {
          const length = type === "write" ? value?.data?.byteLength : value?.length;
          if (!Number.isSafeInteger(value?.offset) || value.offset < 0 || !Number.isSafeInteger(length) || length <= 0 || length > 8 * 1024 ** 2 || value.offset + length > size) throw new Error("Invalid offline range");
          if (type === "read") {
            const bytes = new Uint8Array(length);
            result = bytes.buffer.slice(0, handle.read(bytes, {at: value.offset}));
          } else {
            if (!(value.data instanceof ArrayBuffer) || handle.write(new Uint8Array(value.data), {at: value.offset}) !== length) throw new Error("Offline write incomplete");
            handle.flush();
          }
        }
      }
      self.postMessage({id, value: result}, result instanceof ArrayBuffer ? [result] : []);
    } catch (error) { self.postMessage({id, error: error instanceof Error ? error.message : "Offline storage failed"}); }
  };
}

async function openOfflineWriter(jobID, size) {
  if (!navigator.storage?.getDirectory || !("Worker" in window)) return;
  const url = URL.createObjectURL(new Blob([`(${offlineStorageWorkerScope.toString()})()`], {type: "text/javascript"}));
  const worker = new Worker(url);
  let nextID = 0, failed;
  const pending = new Map();
  const stop = (error) => {
    failed = error;
    for (const request of pending.values()) { clearTimeout(request.timer); request.reject(error); }
    pending.clear();
    worker.terminate();
    URL.revokeObjectURL(url);
  };
  worker.onerror = () => stop(new Error("Offline storage worker failed"));
  worker.onmessage = ({data}) => {
    resolveOfflineWorkerRequest(pending, data);
  };
  const request = (type, value) => new Promise((resolve, reject) => {
    if (failed) { reject(failed); return; }
    const id = nextID++;
    const timer = setTimeout(() => stop(new Error("Offline storage stopped responding")), 30000);
    pending.set(id, {resolve, reject, timer});
    // Clone writes: the incremental digest worker subsequently takes ownership.
    worker.postMessage({id, type, value});
  });
  try {
    if (!await request("open", {jobID, size})) { stop(new Error("Offline writer unsupported")); return; }
  } catch (error) { stop(error); throw error; }
  return {
    read: (offset, length) => request("read", {offset, length}),
    write: (offset, data) => request("write", {offset, data}),
    truncate: () => request("truncate"),
    close: async () => { try { if (!failed) await request("close"); } finally { stop(new Error("Offline writer closed")); } },
  };
}
