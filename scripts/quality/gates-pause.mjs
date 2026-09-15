import { existsSync } from 'node:fs';
if (existsSync(new URL('../../.gates-disabled', import.meta.url))) {
  console.log('Quality gates are disabled until explicitly enabled (.gates-disabled).');
  process.exit(0);
}
