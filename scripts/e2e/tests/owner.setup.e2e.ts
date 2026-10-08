import { test } from '@e2e-dev/web';
import { expect } from 'e2e';
import { totp, requireFixtureURL } from './helpers';
import { readFixtureSetupResponse } from '../fixture-setup.mjs';

test.setup('enroll an Owner with a confirmed second factor', { sessions: ['owner'] }, async ({ app, browser, session }) => {
  requireFixtureURL(app.baseUrl);
  const response = await fetch(new URL('/api/v1/setup', app.baseUrl), { method: 'POST', redirect: 'error', signal: AbortSignal.timeout(10000), headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name: 'Owner', password: 'synthetic-e2e-password', device: 'Disposable browser', totp: true, automaticUpdates: false }) });
  expect(response.status).toBe(201);
  const setup = await readFixtureSetupResponse(response, app.baseUrl);
  const confirmation = await fetch(new URL('/api/v1/me/mfa', app.baseUrl), { method: 'PUT', redirect: 'error', signal: AbortSignal.timeout(10000), headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${setup.token}` }, body: JSON.stringify({ code: totp(setup.totp.secret) }) });
  expect(confirmation.status).toBe(200);
  const onboarding = await fetch(new URL('/api/v1/settings/onboarding', app.baseUrl), { method: 'PUT', redirect: 'error', signal: AbortSignal.timeout(10000), headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${setup.token}` }, body: '{"enabled":false}' });
  expect(onboarding.status).toBe(200);
  await browser.setCookies([{ name: '__Host-kinosail_session', value: setup.token, domain: '127.0.0.1', path: '/', secure: true, httpOnly: true, sameSite: 'Strict' }]);
  await app.open('/settings');
  await expect(browser).toHaveURL('/settings');
  await session.save('owner');
});
