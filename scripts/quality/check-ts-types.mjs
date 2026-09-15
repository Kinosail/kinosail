#!/usr/bin/env node
import './gates-pause.mjs';
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import process from 'node:process';

const repo = resolve(new URL('../..', import.meta.url).pathname);
const nativeManifest = resolve(repo, 'apps/player/apps/native/package.json');
const require = createRequire(nativeManifest);
const ts = require('typescript');
const files = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '*.ts', '*.tsx'], {
  cwd: repo,
  encoding: 'utf8',
}).trim().split('\n').filter(Boolean).filter((file) => !file.includes('/.codex/') && existsSync(resolve(repo, file)));
const failures = [];

for (const file of files) {
  const source = ts.createSourceFile(
    file,
    readFileSync(resolve(repo, file), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    file.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  function visit(node) {
    if (node.kind === ts.SyntaxKind.AnyKeyword || node.kind === ts.SyntaxKind.UnknownKeyword) {
      const position = source.getLineAndCharacterOfPosition(node.getStart(source));
      const type = node.kind === ts.SyntaxKind.AnyKeyword ? 'any' : 'unknown';
      failures.push(`${file}:${position.line + 1}: forbidden TypeScript type ${type}`);
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
}

if (failures.length) {
  console.error(failures.join('\n'));
  process.exitCode = 1;
}
