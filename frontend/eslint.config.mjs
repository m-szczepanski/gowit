import {plugin as shadcn} from '@shadcn/lint';
import tsParser from '@typescript-eslint/parser';
import {defineConfig} from 'eslint/config';

export default defineConfig([
    {
        ignores: ['dist/**', 'coverage/**', 'node_modules/**', 'wailsjs/**']
    },
    {
        files: ['**/*.{js,jsx,ts,tsx}'],
        languageOptions: {
            parser: tsParser,
            parserOptions: {ecmaFeatures: {jsx: true}}
        },
        plugins: {shadcn},
        rules: {
            'shadcn/no-restyle': ['error', {allow: ['layout']}],
            'shadcn/no-raw-colors': 'error',
            'shadcn/no-arbitrary-values': 'error',
            'shadcn/no-inline-styles': 'error',
            'shadcn/no-unknown-classes': 'error',
            'shadcn/require-static-classes': 'error'
        }
    },
    {
        // Vendored registry code: restyled through props, not linted here.
        files: ['src/components/ui/**'],
        rules: {
            'shadcn/no-restyle': 'off',
            'shadcn/no-arbitrary-values': 'off',
            'shadcn/no-unknown-classes': 'off'
        }
    },
    {
        // Tests pass class strings to cn() as data, not as styling.
        files: ['src/**/*.test.{ts,tsx}'],
        rules: {
            'shadcn/no-restyle': 'off',
            'shadcn/no-raw-colors': 'off',
            'shadcn/no-arbitrary-values': 'off',
            'shadcn/no-unknown-classes': 'off',
            'shadcn/require-static-classes': 'off'
        }
    }
]);
