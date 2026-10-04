import {render, screen, waitFor} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import App from './App';

vi.mock('../wailsjs/go/main/App', () => ({
    ExampleBind: vi.fn()
}));

describe('App shell', () => {
    it('renders the sidebar and main panel placeholders', () => {
        render(<App/>);
        expect(screen.getByTestId('sidebar').textContent).toBe('Sidebar');
        expect(screen.getByTestId('bind-result').textContent).toBe('Go binding not called yet');
    });

    it('shows the Go response after the ExampleBind round-trip', async () => {
        const {ExampleBind} = await import('../wailsjs/go/main/App');
        (ExampleBind as ReturnType<typeof vi.fn>).mockResolvedValue('gowit backend is reachable');

        render(<App/>);
        screen.getByText('Ping Go backend').click();

        await waitFor(() =>
            expect(screen.getByTestId('bind-result').textContent).toBe('gowit backend is reachable')
        );
    });

    it('shows the error when the binding call fails', async () => {
        const {ExampleBind} = await import('../wailsjs/go/main/App');
        (ExampleBind as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('no wails runtime'));

        render(<App/>);
        screen.getByText('Ping Go backend').click();

        await waitFor(() =>
            expect(screen.getByTestId('bind-result').textContent).toBe('Error: no wails runtime')
        );
    });
});
