import test from 'node:test';
import assert from 'node:assert/strict';
import { parseCode, validateRun } from '../contracts.mjs';
test('reads only one strictly formed dynamic pairing code, including iOS spoken digits', () => {
  assert.equal(parseCode(['Approval code 8 3 9 2 7 1']), '839271');
  assert.equal(parseCode(['839271']), '839271');
  for (const values of [[], ['12345'], ['1234567'], ['123456', '654321'], ['approval code abcdef'], ['123456\nsecret'], ['x'.repeat(2049)]]) assert.throws(() => parseCode(values));
});
test('rejects ambiguous, unknown, oversized, physical and malformed device/run inputs before lifecycle side effects', () => {
  const good = { platform: 'ios', device: '12345678-1234-1234-1234-123456789ABC', port: '18769', revision: 'a'.repeat(40), run: '12345-1' };
  assert.equal(validateRun(good).platform, 'ios');
  assert.equal(validateRun({ ...good, platform: 'android', device: 'emulator-5554' }).platform, 'android');
  for (const bad of [{platform:'tvos'}, {platform:'watchos'}, {platform:''}, {device:'personal phone'}, {device:'emulator-5555'}, {port:'80'}, {port:'18769oops'}, {port:'70000'}, {run:'../escape'}, {revision:'main'}, {unknown:'x'}, {device:'x'.repeat(2049)}]) assert.throws(() => validateRun({ ...good, ...bad }));
});
