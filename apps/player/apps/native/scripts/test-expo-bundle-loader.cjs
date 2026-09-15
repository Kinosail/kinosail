const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { dirname, join } = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const source = readFileSync(
  join(
    dirname(require.resolve('expo/package.json')),
    'src/async-require/fetchThenEvalJs.ts',
  ),
  'utf8',
);
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS },
}).outputText;

function loader(response) {
  const context = vm.createContext({
    exports: {},
    process: { env: { NODE_ENV: 'production' } },
    require(name) {
      if (name === './fetchAsync') return { fetchAsync: async () => response };
      if (name === './errors') return { MetroServerError: Error };
      throw new Error(`Unexpected import: ${name}`);
    },
  });
  vm.runInContext(compiled, context);
  return { context, load: context.exports.fetchThenEvalAsync };
}

test('split bundles register globally without access to loader local variables', async () => {
  const { context, load } = loader({
    status: 200,
    body: 'globalThis.registeredBundle = 42; globalThis.loaderBodyType = typeof body;',
  });
  await load('https://example.invalid/chunk.bundle');
  assert.equal(context.registeredBundle, 42);
  assert.equal(context.loaderBodyType, 'undefined');
});

test('HTTP failures reject without evaluating the response', async () => {
  const { context, load } = loader({
    status: 500,
    body: 'globalThis.executed = true;',
  });
  await assert.rejects(
    load('https://example.invalid/chunk.bundle'),
    /Failed to load split bundle/,
  );
  assert.equal(context.executed, undefined);
});

test('JSON error responses reject without evaluating the body', async () => {
  const { context, load } = loader({
    status: 200,
    headers: new Map([['Content-Type', 'application/json']]),
    body: JSON.stringify({ message: 'bundle unavailable' }),
  });
  await assert.rejects(
    load('https://example.invalid/chunk.bundle'),
    /bundle unavailable/,
  );
  assert.equal(context.registeredBundle, undefined);
});

test('bundle syntax and runtime failures remain rejected promises', async () => {
  for (const body of [
    'function {',
    'throw new Error("bundle runtime failure")',
  ]) {
    const { load } = loader({ status: 200, body });
    await assert.rejects(load('https://example.invalid/chunk.bundle'));
  }
});
