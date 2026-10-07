// Record only bounded media bytes and range metadata, never private URLs.
import {createHash} from 'node:crypto';

export function observeDirectDelivery(page, id, network, pending) {
  network.directResponses = [];
  let responses = 0;
  page.on('response', response => {
    const path = new URL(response.url()).pathname;
    if (!path.startsWith('/media/') && !path.startsWith('/hls/')) return;
    if (path !== `/media/${id}`) {network.unexpectedMediaRequests++; return;}
    if (++responses > 32 || ![200, 206].includes(response.status())) {
      network.failedMediaResponses++;
      return;
    }
    const headers = response.headers();
    const length = Number(headers['content-length']);
    const contentRange = headers['content-range'] || '';
    if (!Number.isSafeInteger(length) || length <= 0 || length > 8 * 1024 * 1024 ||
        contentRange.length > 80) {network.failedMediaResponses++; return;}
    pending.push(response.body().then(body => {
      if (body.length !== length) throw new Error('direct_response_length');
      network.directResponses.push({status: response.status(), contentRange, bytes: body.length,
        sha256: createHash('sha256').update(body).digest('hex')});
    }).catch(() => {network.failedMediaResponses++;}));
  });
}

export async function resetProofResume(context, input, deadline) {
  const remaining = deadline - Date.now();
  if (!/^http:\/\/localhost:[1-9][0-9]{0,4}$/.test(input.url) || !/^[a-f0-9]{16}$/.test(input.itemID) ||
      !/^[a-zA-Z0-9_.-]{16,2048}$/.test(input.token) || input.resumeSeconds !== 12.5 ||
      !Number.isFinite(remaining) || remaining <= 0 || remaining > 240000) throw new Error('renderer_resume_reset_scope');
  const response = await context.request.put(`${input.url}/api/v1/items/${input.itemID}/progress`, {
    headers: {Authorization: `Bearer ${input.token}`}, data: {seconds: input.resumeSeconds},
    timeout: Math.min(5000, remaining), maxRedirects: 0});
  const data = await response.body();
  if (response.status() !== 200 || data.length > 4096 || Date.now() >= deadline ||
      JSON.parse(data).seconds !== input.resumeSeconds) throw new Error('renderer_resume_reset_failed');
  return true;
}
