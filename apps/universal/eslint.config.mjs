import js from '@eslint/js';
import tsParser from '@typescript-eslint/parser';
import tsPlugin from '@typescript-eslint/eslint-plugin';
import reactPlugin from 'eslint-plugin-react';
import reactHooksPlugin from 'eslint-plugin-react-hooks';

export default [
  js.configs.recommended,
  {
    files: ['**/*.{ts,tsx}'],
    plugins: {
      '@typescript-eslint': tsPlugin,
      react: reactPlugin,
      'react-hooks': reactHooksPlugin
    },
    languageOptions: {
      parser: tsParser,
      parserOptions: {}
    },
    rules: {
      ...tsPlugin.configs.recommended.rules,
      ...reactPlugin.configs.recommended.rules,
      ...reactHooksPlugin.configs.recommended.rules,
      '@typescript-eslint/no-explicit-any': 'off',
      '@typescript-eslint/no-empty-object-type': 'off',
      'no-undef': 'off',
      'react/react-in-jsx-scope': 'off',
      'no-restricted-syntax': [
        'error',
        {
          selector: "JSXOpeningElement[name.type='JSXIdentifier'][name.name='span']",
          message: 'Use React Native/Tamagui text primitives instead of raw <span>.'
        }
      ]
    },
    settings: {
      react: {
        version: 'detect'
      }
    }
  },
  {
    ignores: ['dist/**', '.expo/**', 'node_modules/**', '*.config.*', 'babel.config.js']
  },
  {
    files: ['**/*.test.ts'],
    rules: {
      'no-undef': 'off'
    }
  }
];
