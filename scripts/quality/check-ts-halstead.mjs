#!/usr/bin/env node
import './gates-pause.mjs';
import { execFileSync } from 'node:child_process';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import process from 'node:process';
import { createScanner, forEachSourceFile, is, lineOf, SyntaxKind } from './typescript-source.mjs';

const limit = 80;
const repo = resolve(new URL('../..', import.meta.url).pathname);
const operands = new Set([
  SyntaxKind.Identifier,
  SyntaxKind.PrivateIdentifier,
  SyntaxKind.NumericLiteral,
  SyntaxKind.BigIntLiteral,
  SyntaxKind.StringLiteral,
  SyntaxKind.NoSubstitutionTemplateLiteral,
  SyntaxKind.RegularExpressionLiteral,
  SyntaxKind.TrueKeyword,
  SyntaxKind.FalseKeyword,
  SyntaxKind.NullKeyword,
  SyntaxKind.ThisKeyword,
  SyntaxKind.SuperKeyword,
]);
const structure = new Set([
  SyntaxKind.OpenBraceToken,
  SyntaxKind.CloseBraceToken,
  SyntaxKind.OpenParenToken,
  SyntaxKind.CloseParenToken,
  SyntaxKind.OpenBracketToken,
  SyntaxKind.CloseBracketToken,
  SyntaxKind.CommaToken,
  SyntaxKind.SemicolonToken,
  SyntaxKind.ColonToken,
  SyntaxKind.EndOfFile,
]);
const productionFile = (file) =>
  !file.includes('/third_party/') &&
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
  is.isFunctionDeclaration(node) ||
  is.isFunctionExpression(node) ||
  is.isArrowFunction(node) ||
  is.isMethodDeclaration(node) ||
  is.isConstructorDeclaration(node) ||
  is.isGetAccessorDeclaration(node) ||
  is.isSetAccessorDeclaration(node);

const functionName = (node) => {
  if (node.name) return node.name.getText();
  if (is.isVariableDeclaration(node.parent) && node.parent.name) {
    return node.parent.name.getText();
  }
  return '<anonymous>';
};

const difficulty = (text, variant) => {
  const scanner = createScanner(true, variant, text);
  const distinctOperators = new Set();
  const distinctOperands = new Set();
  let totalOperands = 0;
  for (let token = scanner.scan(); token !== SyntaxKind.EndOfFile; token = scanner.scan()) {
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

forEachSourceFile(repo, files, (file, source) => {
  const visit = (node) => {
    if (isFunction(node) && node.body) {
      const value = difficulty(node.body.getText(), source.languageVariant);
      if (value >= limit) {
        failures.push(
          `${file}:${lineOf(source, node)}: ${functionName(node)} has Halstead difficulty ${value.toFixed(2)}; maximum is less than ${limit}`,
        );
      }
    }
    node.forEachChild(visit);
  };
  visit(source);
});

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exitCode = 1;
}
