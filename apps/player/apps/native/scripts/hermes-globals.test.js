const { mkdtempSync, writeFileSync, rmSync } = require('node:fs');
const { join, dirname } = require('node:path');
const { tmpdir } = require('node:os');
const { createRequire } = require('node:module');
const { spawnSync } = require('node:child_process');

it('declares host globals without hiding an unknown JavaScript identifier', () => {
  const fromReactNative = createRequire(
    require.resolve('react-native/package.json'),
  );
  const compilerRoot = dirname(
    fromReactNative.resolve('hermes-compiler/package.json'),
  );
  const executable =
    process.platform === 'darwin'
      ? 'osx-bin/hermesc'
      : process.platform === 'win32'
        ? 'win64-bin/hermesc.exe'
        : 'linux64-bin/hermesc';
  const directory = mkdtempSync(join(tmpdir(), 'kinosail-hermes-globals-'));
  try {
    const input = join(directory, 'input.js');
    writeFileSync(
      input,
      '"use strict"; Promise.resolve(fetch("https://example.invalid")); unknownKinosailGlobal();',
    );
    const result = spawnSync(
      join(compilerRoot, 'hermesc', executable),
      [
        '-emit-binary',
        '-O',
        `-include-globals=${join(__dirname, 'hermes-globals.js')}`,
        '-out',
        join(directory, 'output.hbc'),
        input,
      ],
      { encoding: 'utf8' },
    );
    expect(result.status).toBe(0);
    expect(result.stderr).toContain('unknownKinosailGlobal');
    expect(result.stderr).not.toContain('variable "Promise"');
    expect(result.stderr).not.toContain('variable "fetch"');
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});
