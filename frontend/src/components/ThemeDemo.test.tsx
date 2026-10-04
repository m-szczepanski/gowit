import {render, screen, within} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import {ThemeDemo} from './ThemeDemo';

describe('ThemeDemo', () => {
    it('renders the primitive set in both a light and a dark token panel', () => {
        render(<ThemeDemo/>);

        const light = screen.getByTestId('theme-demo-light');
        const dark = screen.getByTestId('theme-demo-dark');

        for (const panel of [light, dark]) {
            expect(within(panel).getByRole('button', {name: 'Primary'})).toBeTruthy();
            expect(within(panel).getByText('+ added line')).toBeTruthy();
            expect(within(panel).getByText('- removed line')).toBeTruthy();
            expect(within(panel).getByText('success')).toBeTruthy();
        }
    });
});
