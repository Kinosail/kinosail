#!/usr/bin/env node
import './gates-pause.mjs';
import { readFileSync } from 'node:fs';
import process from 'node:process';

const report = JSON.parse(readFileSync(process.argv[2], 'utf8'));
const unresolved = [];
let executable = 0;
let total = 0;

for (const file of Object.values(report.files ?? {})) {
  for (const mutant of file.mutants ?? []) {
    total += 1;
    const justifiedEquivalent =
      mutant.status === 'Ignored' && mutant.statusReason?.trim();
    if (!justifiedEquivalent) executable += 1;
    if (
      mutant.status !== 'Killed' &&
      mutant.status !== 'Timeout' &&
      mutant.status !== 'CompileError' &&
      !justifiedEquivalent
    ) {
      unresolved.push(`${file.source ?? 'unknown source'}:${mutant.location?.start?.line ?? 0}: ${mutant.status}`);
    }
  }
}

if (total === 0 || executable === 0) {
  console.error('native mutation check did not execute any mutants');
  process.exitCode = 1;
}
if (unresolved.length > 0) {
  console.error(`native mutation check has ${unresolved.length} unresolved mutants`);
  console.error(unresolved.join('\n'));
  process.exitCode = 1;
}
