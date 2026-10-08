import {requireFixtureURL, decodeFixtureJSON} from './fixture-response.mjs';

// Only the synthetic Owner enrollment requested by owner.setup.e2e.ts is admitted.
export async function readFixtureSetupResponse(response, baseURL) {
  const origin = requireFixtureURL(baseURL);
  if (response.status !== 201 || response.url !== origin + '/api/v1/setup') throw new Error('owned SDK setup response required');
  const length = response.headers.get('content-length');
  if (length !== null && (!/^\d{1,6}$/.test(length) || Number(length) > 524288)) throw new Error('SDK setup response too large');
  if (!/^application\/json(?:\s*;|$)/i.test(response.headers.get('content-type') ?? '')) throw new Error('SDK setup JSON required');
  const reader = response.body?.getReader();
  if (!reader) throw new Error('SDK setup response missing');
  const chunks = []; let size = 0;
  try {
    for (;;) {
      const {done, value} = await reader.read(); if (done) break;
      size += value.byteLength; if (size > 524288) throw new Error('SDK setup response too large');
      chunks.push(value);
    }
  } catch (error) {await reader.cancel().catch(() => {}); throw error;}
  finally {reader.releaseLock();}
  const bytes = new Uint8Array(size); let offset = 0;
  for (const chunk of chunks) {bytes.set(chunk, offset); offset += chunk.byteLength;}
  const data = decodeFixtureJSON(new TextDecoder('utf-8', {fatal: true}).decode(bytes));
  const keys = (value, names) => value !== null && typeof value === 'object' && !Array.isArray(value)
    && Object.keys(value).length === names.length && names.every(name => Object.hasOwn(value, name));
  if (!keys(data, ['token', 'expiresIn', 'mfaEnrollmentRequired', 'totp'])
      || typeof data.token !== 'string' || !/^[\x21-\x7e]{1,256}$/.test(data.token)
      || data.expiresIn !== 2592000 || data.mfaEnrollmentRequired !== true
      || !keys(data.totp, ['secret', 'uri', 'recoveryCodes'])
      || typeof data.totp.secret !== 'string' || !/^[A-Z2-7]{32}$/.test(data.totp.secret)
      || data.totp.uri !== `otpauth://totp/Kinosail:Owner?secret=${data.totp.secret}&issuer=Kinosail&digits=6&period=30`
      || !Array.isArray(data.totp.recoveryCodes) || data.totp.recoveryCodes.length !== 10
      || data.totp.recoveryCodes.some(code => typeof code !== 'string' || !/^[A-F0-9]{4}(?:-[A-F0-9]{4}){3}$/.test(code))
      || new Set(data.totp.recoveryCodes).size !== 10) throw new Error('invalid SDK Owner enrollment');
  return data;
}
