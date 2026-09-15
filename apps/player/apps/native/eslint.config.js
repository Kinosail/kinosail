const { defineConfig } = require('eslint/config');
const expoConfig = require('eslint-config-expo/flat');

module.exports = defineConfig([
  expoConfig,
  { ignores: ['.stryker-tmp/**', '.verification/**'] },
  {
    ignores: ['.expo/**', 'android/**', 'coverage/**', 'dist/**', 'ios/**'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'CallExpression[callee.object.name="console"]',
          message: 'Do not write session or media data to the console.',
        },
      ],
    },
  },
  {
    files: ['**/*.test.js'],
    languageOptions: { globals: { expect: 'readonly', it: 'readonly' } },
  },
  {
    files: ['scripts/**/*.mjs'],
    languageOptions: { globals: { Buffer: 'readonly' } },
  },
]);
