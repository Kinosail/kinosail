import importPlugin from 'eslint-plugin-import';
import globals from 'globals';

// Retain the web rules formerly inherited through the retired Expo package.
export default [
  importPlugin.flatConfigs.recommended,
  importPlugin.flatConfigs.errors,
  {
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.browser,
        // Supplied by the embedded HLS library and the on-demand Cast sender.
        Hls: 'readonly',
        cast: 'readonly',
        chrome: 'readonly',
        exports: 'readonly',
        global: 'readonly',
        module: 'readonly',
        process: 'readonly',
        require: 'readonly',
      },
    },
    settings: {
      'import/extensions': ['.js', '.mjs', '.cjs'],
      'import/resolver': { node: { extensions: ['.js', '.mjs', '.cjs'] } },
    },
    rules: {
      eqeqeq: ['warn', 'smart'],
      'no-dupe-args': 'error',
      'no-dupe-class-members': 'error',
      'no-dupe-keys': 'error',
      'no-duplicate-case': 'error',
      'no-empty-character-class': 'warn',
      'no-empty-pattern': 'warn',
      'no-extend-native': 'warn',
      'no-extra-bind': 'warn',
      'no-redeclare': 'warn',
      'no-undef': 'error',
      'no-unreachable': 'warn',
      'no-unsafe-negation': 'warn',
      'no-unused-expressions': ['warn', { allowShortCircuit: true, enforceForJSX: true }],
      'no-unused-labels': 'warn',
      'no-unused-vars': ['warn', {
        vars: 'all', args: 'none', ignoreRestSiblings: true,
        caughtErrors: 'all', caughtErrorsIgnorePattern: '^_',
      }],
      'no-with': 'warn',
      'unicode-bom': ['warn', 'never'],
      'use-isnan': 'error',
      'valid-typeof': 'error',
      'import/first': 'warn',
      'import/default': 'off',
      'no-var': 'error',
      'no-restricted-syntax': ['error', {
        selector: 'CallExpression[callee.object.name="console"]',
        message: 'Do not write session or media data to the console.',
      }],
    },
  },
];
