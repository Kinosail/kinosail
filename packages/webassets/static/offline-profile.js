
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
