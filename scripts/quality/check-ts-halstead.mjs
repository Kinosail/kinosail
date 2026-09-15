#!/usr/bin/env node
import './gates-pause.mjs';
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import process from 'node:process';

const limit = 80;
const repo = resolve(new URL('../..', import.meta.url).pathname);
const manifest = resolve(repo, 'scripts/quality/package.json');
const require = createRequire(manifest);
const ts = require('typescript');
const operands = new Set([
  ts.SyntaxKind.Identifier,
  ts.SyntaxKind.PrivateIdentifier,
  ts.SyntaxKind.NumericLiteral,
  ts.SyntaxKind.BigIntLiteral,
  ts.SyntaxKind.StringLiteral,
  ts.SyntaxKind.NoSubstitutionTemplateLiteral,
  ts.SyntaxKind.RegularExpressionLiteral,
  ts.SyntaxKind.TrueKeyword,
  ts.SyntaxKind.FalseKeyword,
  ts.SyntaxKind.NullKeyword,
  ts.SyntaxKind.ThisKeyword,
  ts.SyntaxKind.SuperKeyword,
]);
const structure = new Set([
  ts.SyntaxKind.OpenBraceToken,
  ts.SyntaxKind.CloseBraceToken,
  ts.SyntaxKind.OpenParenToken,
  ts.SyntaxKind.CloseParenToken,
  ts.SyntaxKind.OpenBracketToken,
  ts.SyntaxKind.CloseBracketToken,
  ts.SyntaxKind.CommaToken,
  ts.SyntaxKind.SemicolonToken,
  ts.SyntaxKind.ColonToken,
  ts.SyntaxKind.EndOfFileToken,
]);
const productionFile = (file) =>
  !file.endsWith('.d.ts') &&
  !file.match(/\.(test|spec)\.[^.]+$/) &&
  !file.match(/\/(hls|htmx)\.min\.js$/) &&
  (/^apps\/[^/]+\/internal\/.*\.js$/.test(file) ||
    /^packages\/webassets\/static\/.*\.js$/.test(file));

const files = execFileSync(
  'git',
  [
    'ls-files',
    '--cached',
    '--others',
    '--exclude-standard',
    '*.js',
    '*.ts',
    '*.tsx',
  ],
  { cwd: repo, encoding: 'utf8' },
)
  .trim()
  .split('\n')
  .filter(Boolean)
  .filter((file) => productionFile(file) && existsSync(resolve(repo, file)));
const failures = [];

const isFunction = (node) =>
  ts.isFunctionDeclaration(node) ||
  ts.isFunctionExpression(node) ||
  ts.isArrowFunction(node) ||
  ts.isMethodDeclaration(node) ||
  ts.isConstructorDeclaration(node) ||
  ts.isGetAccessorDeclaration(node) ||
  ts.isSetAccessorDeclaration(node);

const functionName = (node, source) => {
  if (node.name) return node.name.getText(source);
  if (ts.isVariableDeclaration(node.parent) && node.parent.name) {
    return node.parent.name.getText(source);
  }
  return '<anonymous>';
};

const difficulty = (text, kind) => {
  const scanner = ts.createScanner(ts.ScriptTarget.Latest, true, kind, text);
  const distinctOperators = new Set();
  const distinctOperands = new Set();
  let totalOperands = 0;
  for (let token = scanner.scan(); token !== ts.SyntaxKind.EndOfFileToken; token = scanner.scan()) {
    if (operands.has(token)) {
      distinctOperands.add(`${token}:${scanner.getTokenText()}`);
      totalOperands += 1;
    } else if (!structure.has(token)) {
      distinctOperators.add(token);
    }
  }
  if (distinctOperands.size === 0) return 0;
  return (
    (distinctOperators.size / 2) * (totalOperands / distinctOperands.size)
  );
};

for (const file of files) {
  const source = ts.createSourceFile(
    file,
    readFileSync(resolve(repo, file), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    file.endsWith('.tsx')
      ? ts.ScriptKind.TSX
      : file.endsWith('.ts')
        ? ts.ScriptKind.TS
        : ts.ScriptKind.JS,
  );
  const visit = (node) => {
    if (isFunction(node) && node.body) {
      const value = difficulty(node.body.getText(source), source.languageVariant);
      if (value >= limit) {
        const position = source.getLineAndCharacterOfPosition(node.getStart(source));
        failures.push(
          `${file}:${position.line + 1}: ${functionName(node, source)} has Halstead difficulty ${value.toFixed(2)}; maximum is less than ${limit}`,
        );
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
}

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exitCode = 1;
}
