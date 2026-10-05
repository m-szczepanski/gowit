import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import App from './App';
import {useRepoStore} from '@/stores/repo';
import {useUiStore} from '@/stores/ui';

vi.mock('../wailsjs/go/main/App', () => ({
    OpenFolder: vi.fn(),
    OpenRepository: vi.fn(),
    GetSettings: vi.fn(),
    SetSettings: vi.fn(),
    GetRecentRepos: vi.fn(),
    AddRecentRepo: vi.fn()
}));

type BindingName = 'OpenFolder' | 'OpenRepository' | 'GetSettings' | 'SetSettings' | 'GetRecentRepos' | 'AddRecentRepo';

async function binding(name: BindingName) {
    const mod = await import('../wailsjs/go/main/App');
    return mod[name] as unknown as ReturnType<typeof vi.fn>;
}

// flushes the startup effect promises inside act, keeping RTL quiet
async function renderApp() {
    const result = render(<App/>);
    await act(async () => {
    });
    return result;
}

// jsdom PointerEvents lack pointerType, which Radix's trigger gating
// requires; replay a real mouse sequence instead of userEvent.click
function openMenu(trigger: HTMLElement) {
    fireEvent.pointerDown(trigger, {button: 0, pointers: 1, pointerType: 'mouse'});
    fireEvent.pointerUp(trigger, {button: 0, pointers: 1, pointerType: 'mouse'});
    fireEvent.click(trigger);
}

describe('App shell', () => {
    beforeEach(async () => {
        useRepoStore.getState().closeRepo();
        useUiStore.getState().setActivePanel('status');
        document.documentElement.classList.remove('dark');
        for (const name of ['OpenFolder', 'OpenRepository', 'GetSettings', 'GetRecentRepos', 'AddRecentRepo'] as const) {
            (await binding(name)).mockReset();
        }
        (await binding('GetSettings')).mockResolvedValue({theme: 'dark'});
        (await binding('GetRecentRepos')).mockResolvedValue([]);
        (await binding('AddRecentRepo')).mockResolvedValue({code: ''});
        (await binding('OpenFolder')).mockResolvedValue({path: ''});
        (await binding('OpenRepository')).mockResolvedValue({code: '', path: '/default/repo'});
    });

    it('applies the persisted theme to the document on load', async () => {
        const {unmount} = await renderApp();
        await waitFor(() => expect(document.documentElement.classList.contains('dark')).toBe(true));
        unmount();

        (await binding('GetSettings')).mockResolvedValue({theme: 'system'});
        await renderApp();
        await waitFor(() => expect(document.documentElement.classList.contains('dark')).toBe(false));
    });

    it('keeps rendering when the settings load fails', async () => {
        (await binding('GetSettings')).mockRejectedValue(new Error('config unreadable'));
        await renderApp();

        expect(await screen.findByTestId('empty-state')).toBeTruthy();
    });

    it('keeps rendering when the recents load fails', async () => {
        (await binding('GetRecentRepos')).mockRejectedValue(new Error('config unreadable'));
        await renderApp();

        expect(await screen.findByTestId('empty-state')).toBeTruthy();
        expect(screen.queryByTestId('recent-repos')).toBeNull();
    });

    it('shows the empty state with a call to action when no repo is open', async () => {
        await renderApp();

        expect(screen.getByRole('banner')).toBeTruthy();
        expect(screen.getByRole('main')).toBeTruthy();
        expect(screen.getByRole('contentinfo')).toBeTruthy();
        expect(screen.getByLabelText('Repository panels')).toBeTruthy();
        expect(screen.getByTestId('empty-state').textContent).toContain('No repository open');
        expect(screen.getByTestId('status-branch').textContent).toBe('no repository');
        expect(screen.getByTestId('branches-placeholder').textContent).toContain('Open a repository');
        expect(screen.queryByTestId('recent-repos')).toBeNull();

        for (const action of ['Fetch', 'Pull', 'Push']) {
            expect(screen.getByRole('button', {name: action}).hasAttribute('disabled')).toBe(true);
        }
    });

    it('lists recent repos and reopens one with a click', async () => {
        (await binding('GetRecentRepos')).mockResolvedValue([
            {path: '/home/dev/alpha', lastOpened: '2026-03-01T09:00:00Z'},
            {path: '/home/dev/beta', lastOpened: '2026-02-28T09:00:00Z'}
        ]);
        (await binding('OpenRepository')).mockResolvedValue({code: '', path: '/home/dev/beta'});
        await renderApp();

        const list = await screen.findByTestId('recent-repos');
        expect(list.textContent).toContain('/home/dev/alpha');

        fireEvent.click(screen.getByRole('button', {name: '/home/dev/beta'}));

        await waitFor(() => expect(screen.getByTestId('staging-view')).toBeTruthy());
        expect(useRepoStore.getState().repoPath).toBe('/home/dev/beta');
        expect(await binding('OpenRepository')).toHaveBeenCalledWith('/home/dev/beta');
    });

    it('shows the backend validation hint when the picked folder is not a repo', async () => {
        const hint = '"/srv/code" is not a repository, but /srv/code/gowit are - open one of them';
        (await binding('OpenFolder')).mockResolvedValue({path: '/srv/code'});
        (await binding('OpenRepository')).mockResolvedValue({code: 'not_a_repository', message: hint});
        render(<App/>);

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        expect((await screen.findByTestId('open-error')).textContent).toBe(hint);
        expect(screen.getByTestId('empty-state')).toBeTruthy();
        expect(useRepoStore.getState().repoPath).toBeNull();
    });

    it('reopens a recent repository from the header menu and refreshes the list', async () => {
        const recentGet = (await binding('GetRecentRepos')).mockResolvedValue([
            {path: '/home/dev/alpha', lastOpened: '2026-03-01T09:00:00Z'}
        ]);
        (await binding('OpenRepository')).mockResolvedValue({code: '', path: '/home/dev/alpha'});
        render(<App/>);

        openMenu(screen.getByRole('button', {name: 'Open'}));
        fireEvent.click(await screen.findByRole('menuitem', {name: '/home/dev/alpha'}));

        await waitFor(() => expect(screen.getByTestId('staging-view')).toBeTruthy());
        expect(await binding('OpenRepository')).toHaveBeenCalledWith('/home/dev/alpha');
        expect(recentGet.mock.calls.length).toBeGreaterThan(1);
    });

    it('toasts dialog failures from the header browse action', async () => {
        (await binding('OpenFolder')).mockResolvedValue({code: 'dialog_failed', message: 'native dialog unavailable'});
        render(<App/>);

        openMenu(screen.getByRole('button', {name: 'Open'}));
        fireEvent.click(await screen.findByRole('menuitem', {name: 'Browse folders…'}));

        expect(await screen.findByText('native dialog unavailable')).toBeTruthy();
        expect(screen.queryByTestId('open-error')).toBeNull();
    });

    it('ignores a cancelled browse from the header', async () => {
        (await binding('OpenFolder')).mockResolvedValue({path: ''});
        render(<App/>);

        openMenu(screen.getByRole('button', {name: 'Open'}));
        fireEvent.click(await screen.findByRole('menuitem', {name: 'Browse folders…'}));

        await waitFor(() => expect(screen.queryByRole('menuitem')).toBeNull());
        expect(screen.getByTestId('empty-state')).toBeTruthy();
        expect(await binding('OpenRepository')).not.toHaveBeenCalled();
    });

    it('reports a transport failure when reopening a recent repo', async () => {
        (await binding('GetRecentRepos')).mockResolvedValue([{path: '/z', lastOpened: ''}]);
        (await binding('OpenRepository')).mockRejectedValue(new Error('ipc down'));
        render(<App/>);

        fireEvent.click(await screen.findByRole('button', {name: '/z'}));

        expect((await screen.findByTestId('open-error')).textContent).toBe('Error: ipc down');
    });

    it('falls back to a generic message when a dialog error carries none', async () => {
        (await binding('OpenFolder')).mockResolvedValue({code: 'dialog_failed'});
        render(<App/>);

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        expect((await screen.findByTestId('open-error')).textContent).toBe('failed to open repository');
    });

    it('opens the selected repository through the CTA and swaps in the main tabs', async () => {
        (await binding('OpenFolder')).mockResolvedValue({path: '/home/user/project'});
        (await binding('OpenRepository')).mockResolvedValue({code: '', path: '/home/user/project'});
        await renderApp();

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        await waitFor(() => expect(screen.getByTestId('staging-view')).toBeTruthy());
        expect(screen.getByRole('heading', {name: 'project'})).toBeTruthy();
        expect(screen.getByTestId('branch-indicator').textContent).toBe('no branch info');
        expect(screen.getByTestId('status-branch').textContent).toBe('branch: n/a');
        expect(screen.getByTestId('ahead-behind').textContent).toContain('↑');
        expect(screen.getByTestId('branches-placeholder').textContent).toBe('No branches loaded');
    });

    it('stays in the empty state when the dialog is cancelled', async () => {
        (await binding('OpenFolder')).mockResolvedValue({path: ''});
        await renderApp();

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));
        await waitFor(() => expect(screen.getByTestId('empty-state')).toBeTruthy());
    });

    it('surfaces typed dialog failures', async () => {
        (await binding('OpenFolder')).mockResolvedValue({path: '', code: 'dialog_failed', message: 'native dialog unavailable'});
        await renderApp();

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        expect((await screen.findByTestId('open-error')).textContent).toBe('native dialog unavailable');
    });

    it('surfaces binding transport errors without crashing', async () => {
        (await binding('OpenFolder')).mockRejectedValue(new Error('binding unavailable'));
        await renderApp();

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        expect((await screen.findByTestId('open-error')).textContent).toBe('Error: binding unavailable');
    });

    it('keeps the UI on the resolved work-tree root, not the picked subdirectory', async () => {
        (await binding('OpenFolder')).mockResolvedValue({path: '/root/sub'});
        (await binding('OpenRepository')).mockResolvedValue({code: '', path: '/root'});
        await renderApp();

        fireEvent.click(screen.getByRole('button', {name: 'Open Folder'}));

        await waitFor(() => expect(useRepoStore.getState().repoPath).toBe('/root'));
        expect(screen.getByRole('heading', {name: 'root'})).toBeTruthy();
    });

    it('switches main panel tabs from the ui store', async () => {
        useRepoStore.getState().openRepo('/repo');
        await renderApp();

        fireEvent.mouseDown(screen.getByRole('tab', {name: 'History'}));
        expect(screen.getByTestId('commit-history').textContent).toContain('No commits');
        expect(screen.getByTestId('diff-viewer').textContent).toContain('Select a commit');

        fireEvent.mouseDown(screen.getByRole('tab', {name: 'Graph'}));
        expect(screen.getByTestId('commit-graph')).toBeTruthy();
        expect(useUiStore.getState().activePanel).toBe('graph');
    });

    it('collapses and re-expands the sidebar with the header toggle', async () => {
        await renderApp();
        const toggle = screen.getByRole('button', {name: 'Toggle sidebar'});

        fireEvent.click(toggle);
        expect(toggle.getAttribute('aria-expanded')).toBe('false');

        fireEvent.click(toggle);
        expect(toggle.getAttribute('aria-expanded')).toBe('true');
    });

    it('falls back to the raw path when it has no name segment', async () => {
        useRepoStore.getState().openRepo('/');
        await renderApp();
        expect(screen.getByRole('heading', {name: '/'})).toBeTruthy();
    });
});
