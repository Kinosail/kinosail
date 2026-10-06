import { test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { totp } from './helpers';

test.setup('enroll an Owner with a confirmed second factor', { sessions: ['owner'] }, async ({ app, browser, session }) => {
  const response = await fetch(new URL('/api/v1/setup', app.baseUrl), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: 'Owner', password: 'synthetic-e2e-password', device: 'Disposable browser', totp: true, automaticUpdates: false }) });
  expect(response.status).toBe(201);
  const setup = await response.json();
  const confirmation = await fetch(new URL('/api/v1/me/mfa', app.baseUrl), { method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${setup.token}` }, body: JSON.stringify({ code: totp(setup.totp.secret) }) });
  expect(confirmation.status).toBe(200);
  const onboarding = await fetch(new URL('/api/v1/settings/onboarding', app.baseUrl), { method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${setup.token}` }, body: '{"enabled":false}' });
  expect(onboarding.status).toBe(200);
  await browser.setCookies([{ name: '__Host-kinosail_session', value: setup.token, domain: '127.0.0.1', path: '/', secure: true, httpOnly: true, sameSite: 'Strict' }]);
  await app.open('/settings');
  await expect(browser).toHaveURL('/settings');
  await session.save('owner');
});
