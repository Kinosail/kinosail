const openOfflineDatabaseConnection = () => new Promise((resolve, reject) => {
  const request = indexedDB.open(offlineDatabase, 4);
  request.onupgradeneeded = () => {
    const database = request.result;
    if (!database.objectStoreNames.contains("identity")) database.createObjectStore("identity");
    if (!database.objectStoreNames.contains("jobs")) database.createObjectStore("jobs", {keyPath: "id"});
    const chunks = database.objectStoreNames.contains("chunks") ? request.transaction.objectStore("chunks") : database.createObjectStore("chunks", {keyPath: "id"});
    if (!chunks.indexNames.contains("jobID")) chunks.createIndex("jobID", "jobID");
    if (!chunks.indexNames.contains("jobRange")) chunks.createIndex("jobRange", ["jobID", "offset"]);
  };
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
});

const offlineProfileState = async (profile) => {
  const database = await openOfflineDatabaseConnection();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction("identity", profile === undefined ? "readonly" : "readwrite");
    const store = transaction.objectStore("identity");
    const request = profile === undefined ? store.get("active-profile") : store.put(profile, "active-profile");
    transaction.oncomplete = () => { database.close(); resolve(request.result || ""); };
    transaction.onerror = transaction.onabort = () => { database.close(); reject(transaction.error || request.error); };
  });
};
