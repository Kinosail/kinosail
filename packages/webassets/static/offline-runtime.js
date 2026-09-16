const openOfflineDatabaseConnection = () => new Promise((resolve, reject) => {
  const request = indexedDB.open(offlineDatabase, 3);
  request.onupgradeneeded = () => {
    const database = request.result;
    if (!database.objectStoreNames.contains("jobs")) database.createObjectStore("jobs", {keyPath: "id"});
    const chunks = database.objectStoreNames.contains("chunks") ? request.transaction.objectStore("chunks") : database.createObjectStore("chunks", {keyPath: "id"});
    if (!chunks.indexNames.contains("jobID")) chunks.createIndex("jobID", "jobID");
    if (!chunks.indexNames.contains("jobRange")) chunks.createIndex("jobRange", ["jobID", "offset"]);
  };
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
});
