// Temporary Request-stage observer. This observes actual protocol events and
// continues them unchanged; continuation acknowledgement never proves a drop.
export function createNativeRequestBoundaryDiagnostic(send, endpoint, frame, hasClaim) {
  const counts = {paused: 0, requestStage: 0, endpointPosts: 0, genuineAuthority: 0,
    networkIdPresent: 0, networkIdAbsent: 0, selectedFrame: 0, otherFrame: 0, missingFrame: 0,
    continuationAttempts: 0, continued: 0, continuationFailed: 0,
    observerErrors: 0, invalidEvents: 0, saturated: 0};
  function bump(field) {
    if (counts[field] < 8) counts[field]++;
    else counts.saturated = 1;
  }
  return {async observe(event) {
    let id;
    try {id = event?.requestId;} catch (_) {bump('invalidEvents'); return;}
    if (typeof id !== 'string' || !id) {bump('invalidEvents'); return;}
    bump('paused');
    let continuation;
    // Forward first, without URL, method, header, body or response overrides.
    try {bump('continuationAttempts'); continuation = send('Fetch.continueRequest', {requestId: id});}
    catch (_) {bump('continuationFailed'); return;}
    try {
      const requestStage = !Object.hasOwn(event, 'responseStatusCode') && !Object.hasOwn(event, 'responseErrorReason');
      if (requestStage) bump('requestStage');
      const request = event.request;
      if (requestStage && request?.url === endpoint && request.method === 'POST') {
        bump('endpointPosts');
        const authority = Object.entries(request.headers || {})
          .filter(([name]) => name.toLowerCase() === 'x-kinosail-player-claim');
        if (authority.length === 1 && typeof authority[0][1] === 'string' && hasClaim(authority[0][1])) bump('genuineAuthority');
        if (typeof event.networkId === 'string' && event.networkId) bump('networkIdPresent');
        else bump('networkIdAbsent');
        if (typeof event.frameId !== 'string' || !event.frameId) bump('missingFrame');
        else bump(event.frameId === frame ? 'selectedFrame' : 'otherFrame');
      }
    } catch (_) {bump('observerErrors');}
    try {await continuation; bump('continued');}
    catch (_) {bump('continuationFailed');}
  }, snapshot: () => ({...counts})};
}
