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
