import { once } from 'node:events';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const require = createRequire(new URL('./package.json', import.meta.url));
const { API } = require('typescript/unstable/sync');

export const { SyntaxKind } = require('typescript/unstable/ast');
export const is = require('typescript/unstable/ast/is');
export const { createScanner } = require('typescript/unstable/ast/scanner');

async function shutDown(api, snapshot) {
  snapshot?.dispose();
  const child = api.client?.channel?.child;
  if (child?.stdin && child.exitCode === null) {
    const exited = once(child, 'exit');
    child.stdin.end();
    let timer;
    await Promise.race([exited, new Promise((done) => { timer = setTimeout(done, 5000); })]);
    clearTimeout(timer);
  }
  api.close();
}

export async function forEachSourceFile(repo, files, visit) {
  if (files.length === 0) return;
  const directory = mkdtempSync(join(tmpdir(), 'kinosail-typescript-'));
  const config = join(directory, 'tsconfig.json');
  const paths = files.map((file) => resolve(repo, file));
  writeFileSync(config, JSON.stringify({
    compilerOptions: { allowJs: true, jsx: 'preserve', noEmit: true, noResolve: true, types: [] },
    files: paths,
  }));
  const api = new API({ cwd: repo });
  let snapshot;
  try {
    snapshot = api.updateSnapshot({ openProject: config });
    const program = snapshot.getProjects()[0].program;
    files.forEach((file, index) => visit(file, program.getSourceFile(paths[index])));
  } finally {
    await shutDown(api, snapshot);
    rmSync(directory, { recursive: true, force: true });
  }
}

export function lineOf(source, node) {
  return source.getLineAndCharacterOfPosition(node.getStart()).line + 1;
}
