import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import App from './App';
import {useRepoStore} from '@/stores/repo';
import {useUiStore} from '@/stores/ui';

vi.mock('../wailsjs/go/main/App', () => ({
    OpenFolder: vi.fn()
}));

async function openFolderMock() {
    const {OpenFolder} = await import('../wailsjs/go/main/App');
    return OpenFolder as ReturnType<typeof vi.fn>;
}

describe('App shell', () => {
    beforeEach(() => {
        useRepoStore.getState().closeRepo();
        useUiStore.getState().setActivePanel('status');
    });

    it('shows the empty state with a call to action when no repo is open', async () => {
        render(<App/>);

        expect(screen.getByRole('banner')).toBeTruthy();
        expect(screen.getByRole('contentinfo')).toBeTruthy();
        expect(screen.getByLabelText('Repository panels')).toBeTruthy();
        expect(screen.getByTestId('empty-state').textContent).toContain('No repository open');
        expect(screen.getByTestId('status-branch').textContent).toBe('no repository');
        expect(screen.getByTestId('branches-placeholder').textContent).toContain('Open a repository');

        for (const action of ['Fetch', 'Pull', 'Push']) {
            expect(screen.getByRole('button', {name: action}).hasAttribute('disabled')).toBe(true);
        }
    });

    it('opens the selected repository through the CTA and swaps in the main tabs', async () => {
        (await openFolderMock()).mockResolvedValue('/home/user/project');
        render(<App/>);

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        await waitFor(() => expect(screen.getByTestId('staging-view')).toBeTruthy());
        expect(screen.getByRole('heading', {name: 'project'})).toBeTruthy();
        expect(screen.getByTestId('branch-indicator').textContent).toBe('no branch info');
        expect(screen.getByTestId('status-branch').textContent).toBe('branch: n/a');
        expect(screen.getByTestId('ahead-behind').textContent).toContain('↑');
        expect(screen.getByTestId('branches-placeholder').textContent).toBe('No branches loaded');
    });

    it('stays in the empty state when the dialog is cancelled', async () => {
        (await openFolderMock()).mockResolvedValue('');
        render(<App/>);

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));
        await waitFor(() => expect(screen.getByTestId('empty-state')).toBeTruthy());
    });

    it('surfaces dialog errors without crashing', async () => {
        (await openFolderMock()).mockRejectedValue(new Error('dialog unavailable'));
        render(<App/>);

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        expect((await screen.findByTestId('open-error')).textContent).toBe('Error: dialog unavailable');
    });

    it('switches main panel tabs from the ui store', async () => {
        useRepoStore.getState().openRepo('/repo');
        render(<App/>);

        fireEvent.mouseDown(screen.getByRole('tab', {name: 'History'}));
        expect(screen.getByTestId('commit-history').textContent).toContain('No commits');
        expect(screen.getByTestId('diff-viewer').textContent).toContain('Select a commit');

        fireEvent.mouseDown(screen.getByRole('tab', {name: 'Graph'}));
        expect(screen.getByTestId('commit-graph')).toBeTruthy();
        expect(useUiStore.getState().activePanel).toBe('graph');
    });

    it('collapses and re-expands the sidebar with the header toggle', () => {
        render(<App/>);
        const toggle = screen.getByRole('button', {name: 'Toggle sidebar'});

        fireEvent.click(toggle);
        expect(toggle.getAttribute('aria-expanded')).toBe('false');

        fireEvent.click(toggle);
        expect(toggle.getAttribute('aria-expanded')).toBe('true');
    });

    it('falls back to the raw path when it has no name segment', () => {
        useRepoStore.getState().openRepo('/');
        render(<App/>);
        expect(screen.getByRole('heading', {name: '/'})).toBeTruthy();
    });
});
