#!/usr/bin/env node
import './gates-pause.mjs';
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import process from 'node:process';
import { forEachSourceFile, lineOf, SyntaxKind } from './typescript-source.mjs';

const repo = resolve(new URL('../..', import.meta.url).pathname);
const files = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '*.ts', '*.tsx'], {
  cwd: repo,
  encoding: 'utf8',
}).trim().split('\n').filter(Boolean).filter((file) => !file.includes('/.codex/') && existsSync(resolve(repo, file)));
const failures = [];

await forEachSourceFile(repo, files, (file, source) => {
  function visit(node) {
    if (node.kind === SyntaxKind.AnyKeyword || node.kind === SyntaxKind.UnknownKeyword) {
      const type = node.kind === SyntaxKind.AnyKeyword ? 'any' : 'unknown';
      failures.push(`${file}:${lineOf(source, node)}: forbidden TypeScript type ${type}`);
    }
    node.forEachChild(visit);
  }
  visit(source);
});

if (failures.length) {
  console.error(failures.join('\n'));
  process.exitCode = 1;
}
