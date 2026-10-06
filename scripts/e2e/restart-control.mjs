import { constants, writeFileSync } from 'node:fs';
import { join } from 'node:path';
// Receipt identifiers are observations; only the supervisor selects a process.
export function requestRestart(port, request) {
  if (typeof port !== 'string' || !/^\d{1,5}$/.test(port) || +port < 1024 || +port > 65535 ||
      !request || Object.keys(request).sort().join(',') !== 'childPID,generation' ||
      !Number.isInteger(request.generation) || request.generation < 0 || request.generation >= 3 ||
      !Number.isInteger(request.childPID) || request.childPID <= 1 || request.childPID > 2147483647) throw new Error('invalid fixture restart request');
  writeFileSync(join(process.cwd(), '.e2e/fixtures', port + '.restart'), JSON.stringify(request), {
    flag: constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, mode: 0o600,
  });
}
