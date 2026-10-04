import {act, render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import {useRepoStore} from './repo';
import {useUiStore} from './ui';

function RepoBadge() {
    const isOpen = useRepoStore((s) => s.isOpen);
    const activePanel = useUiStore((s) => s.activePanel);
    return <p data-testid="badge">{isOpen ? `open:${activePanel}` : 'no repo'}</p>;
}

describe('stores in components', () => {
    it('renders reactively from both stores', () => {
        useRepoStore.getState().closeRepo();
        render(<RepoBadge/>);
        expect(screen.getByTestId('badge').textContent).toBe('no repo');

        act(() => {
            useRepoStore.getState().openRepo('/tmp/repo');
            useUiStore.getState().setActivePanel('history');
        });
        expect(screen.getByTestId('badge').textContent).toBe('open:history');

        act(() => useRepoStore.getState().closeRepo());
    });
});
