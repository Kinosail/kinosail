import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint } from 'eslint';
import { browserScriptBundles } from './browser-script-bundles.mjs';

const repo = fileURLToPath(new URL('../../', import.meta.url));
const eslint = new ESLint({ cwd: repo, overrideConfigFile: fileURLToPath(new URL('./eslint.config.mjs', import.meta.url)) });
const bundles = browserScriptBundles(repo);
const bundled = new Set(bundles.flatMap(bundle => bundle.files));
const files = execFileSync('git', ['ls-files', '--cached', '--others', '--exclude-standard', '-z', '*.js', '*.ts', '*.tsx'], { cwd: repo, encoding: 'utf8' }).split('\0').filter(file =>
  /^(apps\/[^/]+\/internal\/|packages\/webassets\/static\/)/.test(file) &&
  !/(?:\/hls\.min\.js|\/htmx\.min\.js|\.d\.ts|\.test\.|\.spec\.)/.test(file) && !bundled.has(file) && existsSync(path.join(repo, file)));
let errors = 0;
let warnings = 0;
for (const bundle of [...bundles, ...files.map(file => ({ name: file, files: [file] }))]) {
  let source = '';
  const origins = [];
  for (const file of bundle.files) {
    const content = readFileSync(path.join(repo, file), 'utf8');
    origins.push({ file, line: source.split('\n').length });
    source += content;
  }
  const [result] = await eslint.lintText(source, { filePath: bundle.files[0] });
  errors += result.errorCount;
  warnings += result.warningCount;
  for (const message of result.messages) {
    const origin = origins.findLast(origin => origin.line <= message.line) || origins[0];
    process.stdout.write(`${origin.file}:${message.line - origin.line + 1}:${message.column}: ${message.severity === 2 ? 'error' : 'warning'} ${message.message} (${message.ruleId || 'parse'}; ${bundle.name})\n`);
  }
}
process.stdout.write(`Browser scripts: ${errors} errors, ${warnings} warnings\n`);
process.exitCode = errors ? 1 : 0;
