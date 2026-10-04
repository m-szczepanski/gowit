import {render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import App from './App';

describe('App shell', () => {
    it('renders sidebar and main panel', () => {
        render(<App/>);
        expect(screen.getByTestId('sidebar').textContent).toBe('Sidebar');
        expect(screen.getByTestId('shell-placeholder').textContent).toBe('gowit shell');
    });
});
