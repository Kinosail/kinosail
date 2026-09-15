import { execFileSync } from 'node:child_process';

const defaultMutations = [
  'src/**/*.{ts,tsx}',
  '!src/**/*.test.{ts,tsx}',
  '!src/**/*.d.ts',
  '!src/testing/**',
];
const mutationBase = process.env.KINOSAIL_MUTATION_DIFF;
const changedMutations = mutationBase
  ? execFileSync(
      'git',
      ['diff', '--name-only', '--relative', mutationBase, '--', 'src'],
      {
        encoding: 'utf8',
      },
    )
      .split('\n')
      .filter(
        (file) =>
          /\.tsx?$/.test(file) &&
          !/\.test\.tsx?$/.test(file) &&
          !file.startsWith('src/testing/'),
      )
      .map((file) => file.replaceAll('[', '[[]'))
  : [];

/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
const config = {
  cleanTempDir: 'always',
  concurrency: 8,
  coverageAnalysis: 'perTest',
  ignorePatterns: ['/ios', '/android', '/dist', '/.verification'],
  jest: {
    enableFindRelatedTests: true,
    projectType: 'custom',
  },
  jsonReporter: {
    fileName: '.verification/mutation.json',
  },
  mutate: changedMutations.length > 0 ? changedMutations : defaultMutations,
  packageManager: 'pnpm',
  plugins: ['@stryker-mutator/jest-runner'],
  reporters: ['clear-text', 'progress', 'json'],
  testRunner: 'jest',
  thresholds: {
    break: 100,
    high: 100,
    low: 100,
  },
  timeoutFactor: 2,
  timeoutMS: 10000,
};

export default config;
