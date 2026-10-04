import {act, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it} from 'vitest';
import {useRepoStore} from './repo';
import {useUiStore} from './ui';

function RepoBadge() {
    const isOpen = useRepoStore((s) => s.isOpen);
    const activePanel = useUiStore((s) => s.activePanel);
    return <p data-testid="badge">{isOpen ? `open:${activePanel}` : 'no repo'}</p>;
}

describe('stores in components', () => {
    beforeEach(() => {
        useRepoStore.getState().closeRepo();
        useUiStore.getState().setActivePanel('status');
        useUiStore.getState().selectCommit(null);
    });

    it('renders reactively from both stores', () => {
        render(<RepoBadge/>);
        expect(screen.getByTestId('badge').textContent).toBe('no repo');

        act(() => {
            useRepoStore.getState().openRepo('/tmp/repo');
            useUiStore.getState().setActivePanel('history');
        });
        expect(screen.getByTestId('badge').textContent).toBe('open:history');
    });
});
