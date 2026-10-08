import {QueryClientProvider} from '@tanstack/react-query';
import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {toast} from 'sonner';
import {StagingView} from '@/components/StagingView';
import {createQueryClient} from '@/lib/queryClient';
import {useRepoStore} from '@/stores/repo';

vi.mock('sonner', () => ({
    toast: {error: vi.fn()}
}));

vi.mock('../../wailsjs/go/main/App', () => ({
    GetStatus: vi.fn(),
    StageFiles: vi.fn(),
    UnstageFiles: vi.fn(),
    StageAll: vi.fn(),
    UnstageAll: vi.fn(),
    DiscardFiles: vi.fn()
}));

type BindingName = 'GetStatus' | 'StageFiles' | 'UnstageFiles' | 'StageAll' | 'UnstageAll' | 'DiscardFiles';

async function binding(name: BindingName) {
    const mod = await import('../../wailsjs/go/main/App');
    return mod[name] as ReturnType<typeof vi.fn>;
}

function file(over: Record<string, unknown> = {}) {
    return {
        xy: '.M', path: 'x', origPath: '', submodule: '',
        untracked: false, ignored: false, conflict: false,
        staged: false, unstaged: true, change: 'modified', stages: [],
        ...over
    };
}

function status(branch: Record<string, unknown> = {}, files: unknown[] = []) {
    return {
        code: '',
        message: '',
        branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0, ...branch},
        files
    };
}

const rich = status(
    {ahead: 2, behind: 1},
    [
        file({path: 'staged.txt', xy: 'M.', staged: true, unstaged: false}),
        file({path: 'work.txt', xy: '.M'}),
        file({path: 'gone.txt', xy: '.D', change: 'deleted'}),
        file({path: 'new.txt', xy: '??', untracked: true, staged: false, unstaged: false, change: 'untracked'}),
        file({path: 'both.txt', xy: 'UU', conflict: true, staged: true, unstaged: true, change: 'conflicted'}),
        file({path: 'moved.txt', origPath: 'old.txt', xy: 'R.', staged: true, unstaged: false, change: 'renamed'}),
        file({path: '.gitignore', xy: '!!', ignored: true, change: 'ignored', unstaged: false})
    ]
);

function renderWith(files: unknown[], branch: Record<string, unknown> = {}) {
    const client = createQueryClient();
    client.setQueryData(['status', '/repo/one'], status(branch, files));
    const Wrapper = ({children}: {children?: ReactNode}) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return render(<StagingView/>, {wrapper: Wrapper});
}

async function mockEcho() {
    const next = status({}, [file({path: 'flipped.txt', xy: 'M.', staged: true, unstaged: false})]);
    for (const name of ['StageFiles', 'UnstageFiles', 'StageAll', 'UnstageAll', 'DiscardFiles'] as BindingName[]) {
        vi.mocked(await binding(name)).mockResolvedValue(next as never);
    }
    return next;
}

describe('StagingView', () => {
    beforeEach(() => {
        useRepoStore.getState().openRepo('/repo/one');
    });

    it('renders branch with ahead/behind in the header', () => {
        renderWith(rich.files, {ahead: 2, behind: 1});
        expect(screen.getByTestId('staging-branch').textContent).toBe('main ↑2 ↓1');
    });

    it('badges rows git marks with a submodule state column', () => {
        renderWith([file({path: 'sub1', xy: '.M', submodule: 'SC..'}), file({path: 'plain.txt'})]);
        expect(screen.getAllByText('sub')).toHaveLength(1);
    });

    it('shows an unborn marker when the branch has no name and no counts', () => {
        renderWith([], {ahead: 0, behind: 0, detached: false, head: '', oid: ''});
        expect(screen.getByTestId('staging-branch').textContent).toBe('(unborn)');
    });

    it('shows detached head with short oid', () => {
        renderWith([], {head: '', detached: true, oid: 'deadbeefcafebabe0000'});
        expect(screen.getByTestId('staging-branch').textContent).toContain('(detached at deadbeef)');
    });

    it('groups rows under Staged, Changes, Untracked and Conflicts, skipping ignored', () => {
        renderWith(rich.files);
        for (const [path, section] of [
            ['staged.txt', 'staged'],
            ['moved.txt', 'staged'],
            ['work.txt', 'changes'],
            ['gone.txt', 'changes'],
            ['new.txt', 'untracked'],
            ['both.txt', 'conflicted']
        ] as const) {
            expect(screen.getByTestId(`row-${section}-${path}`).dataset.section).toBe(section);
        }
        expect(screen.queryByTestId('row-changes-.gitignore')).toBeNull();
        expect(screen.getByTestId('section-staged').textContent).toContain('Staged');
        expect(screen.getByTestId('section-conflicted').textContent).toContain('1');
    });

    it('renders badge, human label and rename display', () => {
        renderWith(rich.files);
        const row = screen.getByTestId('row-changes-work.txt');
        expect(row.textContent).toContain('.M');
        expect(row.textContent).toContain('Modified');
        expect(screen.getByTestId('row-staged-moved.txt').textContent).toContain('old.txt -> moved.txt');
        expect(screen.getByTestId('row-changes-gone.txt').textContent).toContain('Deleted');
    });

    it('shows the empty state when nothing changed', () => {
        renderWith([]);
        expect(screen.getByTestId('staging-empty').textContent).toContain('No local changes');
    });

    it('shows a loading placeholder before the first result', async () => {
        vi.mocked(await binding('GetStatus')).mockReturnValue(new Promise(() => {
        }) as never);
        const Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={createQueryClient()}>{children}</QueryClientProvider>
        );
        render(<StagingView/>, {wrapper: Wrapper});
        expect(screen.getByTestId('staging-loading')).toBeInTheDocument();
    });

    it('toggles a row through the matching mutation and adopts the echo', async () => {
        renderWith(rich.files);
        await mockEcho();

        fireEvent.click(screen.getByRole('checkbox', {name: 'Stage work.txt'}));

        expect(await binding('StageFiles')).toHaveBeenCalledWith(['work.txt']);
        await waitFor(() =>
            expect(screen.getByTestId('row-staged-flipped.txt').dataset.section).toBe('staged')
        );
    });

    it('unstages a checked row', async () => {
        renderWith(rich.files);
        await mockEcho();
        fireEvent.click(screen.getByRole('checkbox', {name: 'Unstage staged.txt'}));
        expect(await binding('UnstageFiles')).toHaveBeenCalledWith(['staged.txt']);
    });

    it('keyboard activation toggles the row checkbox (space and enter)', async () => {
        renderWith(rich.files);
        await mockEcho();
        const checkbox = screen.getByRole('checkbox', {name: 'Stage work.txt'});
        checkbox.focus();
        // jsdom ships no keyboard activation behaviour for buttons, so the
        // browser's space-to-click step is performed explicitly; Radix's
        // own key handling runs for real on top of it
        checkbox.addEventListener('keyup', (e) => {
            if (e.key === ' ') {
                checkbox.click();
            }
        });
        fireEvent.keyDown(checkbox, {key: ' '});
        fireEvent.keyUp(checkbox, {key: ' '});
        expect(await binding('StageFiles')).toHaveBeenCalledWith(['work.txt']);

        const stagedBox = screen.getByRole('checkbox', {name: 'Unstage staged.txt'});
        fireEvent.click(stagedBox);
        expect(await binding('UnstageFiles')).toHaveBeenCalledWith(['staged.txt']);
    });

    it('conflicted checkboxes are disabled and inert', async () => {
        renderWith(rich.files);
        await mockEcho();
        const conflicted = screen.getByRole('checkbox', {name: 'Stage both.txt'});
        expect(conflicted).toBeDisabled();
        fireEvent.click(conflicted);
        expect(await binding('StageFiles')).not.toHaveBeenCalled();
        expect(await binding('UnstageFiles')).not.toHaveBeenCalled();
    });

    it('conflicted rows expose a disabled checkbox', () => {
        renderWith(rich.files);
        expect((screen.getByRole('checkbox', {name: 'Stage both.txt'}) as HTMLInputElement).disabled).toBe(true);
    });

    it('section header select-all stages every file of that section', async () => {
        renderWith(rich.files);
        await mockEcho();
        fireEvent.click(screen.getByRole('checkbox', {name: 'Stage all changes'}));
        expect(await binding('StageFiles')).toHaveBeenCalledWith(['work.txt', 'gone.txt']);
    });

    it('a fully staged section select-all unstages it', async () => {
        renderWith(rich.files);
        await mockEcho();
        fireEvent.click(screen.getByRole('checkbox', {name: 'Unstage all staged'}));
        expect(await binding('UnstageFiles')).toHaveBeenCalledWith(['staged.txt', 'moved.txt']);
    });

    it('conflict section carries no checkbox', () => {
        renderWith(rich.files);
        expect(screen.getByTestId('section-conflicted-placeholder')).toBeInTheDocument();
    });

    it('Stage All and Unstage All call their bindings', async () => {
        renderWith(rich.files);
        await mockEcho();
        fireEvent.click(screen.getByRole('button', {name: 'Stage All'}));
        expect(await binding('StageAll')).toHaveBeenCalled();
        fireEvent.click(screen.getByRole('button', {name: 'Unstage All'}));
        expect(await binding('UnstageAll')).toHaveBeenCalled();
    });

    it('discard asks for confirmation and only then calls the backend', async () => {
        renderWith(rich.files);
        await mockEcho();
        expect(screen.queryByRole('dialog')).toBeNull();
        fireEvent.click(screen.getByRole('button', {name: 'Discard work.txt'}));
        const dialog = await screen.findByRole('dialog');
        expect(dialog.textContent).toContain('work.txt');

        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));
        await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
        expect(await binding('DiscardFiles')).not.toHaveBeenCalled();

        fireEvent.click(screen.getByRole('button', {name: 'Discard work.txt'}));
        await screen.findByRole('dialog');
        fireEvent.click(screen.getByTestId('discard-confirm'));
        expect(await binding('DiscardFiles')).toHaveBeenCalledWith(['work.txt']);
        await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    });

    it('closing the dialog via escape cancels the discard', async () => {
        renderWith(rich.files);
        fireEvent.click(screen.getByRole('button', {name: 'Discard work.txt'}));
        await screen.findByRole('dialog');
        fireEvent.keyDown(document, {key: 'Escape'});
        await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
        expect(await binding('DiscardFiles')).not.toHaveBeenCalled();
    });

    it('disables controls while a mutation is running', async () => {
        let release: (v: unknown) => void = () => {
        };
        vi.mocked(await binding('StageFiles')).mockImplementation(
            () => new Promise((resolve) => {
                release = resolve;
            })
        );
        renderWith(rich.files);
        fireEvent.click(screen.getByRole('checkbox', {name: 'Stage work.txt'}));
        await waitFor(() => expect((screen.getByRole('button', {name: 'Stage All'}) as HTMLButtonElement).disabled).toBe(true));
        release(status({}, []));
        await waitFor(() => expect((screen.getByRole('button', {name: 'Stage All'}) as HTMLButtonElement).disabled).toBe(false));
    });

    it('an MM file appears twice with opposite actions', async () => {
        const files = [...rich.files, file({path: 'dual.txt', xy: 'MM', staged: true, unstaged: true})];
        renderWith(files);
        await mockEcho();
        const stagedRow = screen.getByTestId('row-staged-dual.txt');
        const changesRow = screen.getByTestId('row-changes-dual.txt');
        fireEvent.click(stagedRow.querySelector('button')!);
        expect(await binding('UnstageFiles')).toHaveBeenCalledWith(['dual.txt']);
        fireEvent.click(changesRow.querySelector('button')!);
        expect(await binding('StageFiles')).toHaveBeenCalledWith(['dual.txt']);
        expect((stagedRow.querySelector('button') as HTMLButtonElement).getAttribute('aria-checked')).toBe('true');
        expect(changesRow.querySelector('button')!.getAttribute('aria-checked')).toBe('false');
    });

    it('renders the query error instead of an endless loading state', async () => {
        vi.mocked(await binding('GetStatus')).mockResolvedValue({
            code: 'call_failed', message: 'git died', branch: {}, files: []
        } as never);
        const client = createQueryClient();
        const Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );
        render(<StagingView/>, {wrapper: Wrapper});
        await waitFor(() => expect(screen.getByTestId('staging-error').textContent).toContain('call_failed: git died'));
    });

    it('renders transport failures with a generic code label', async () => {
        vi.mocked(await binding('GetStatus')).mockRejectedValue(new Error('bindings not ready'));
        const client = createQueryClient();
        const Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );
        render(<StagingView/>, {wrapper: Wrapper});
        await waitFor(() => expect(screen.getByTestId('staging-error').textContent).toContain('error: bindings not ready'));
    });

    it('surfaces a failed mutation as a toast', async () => {
        vi.mocked(await binding('StageFiles')).mockResolvedValue({
            code: 'command_failed', message: 'pathspec nope did not match', branch: {}, files: []
        } as never);
        renderWith(rich.files);
        fireEvent.click(screen.getByRole('checkbox', {name: 'Stage work.txt'}));
        await waitFor(() => expect(toast.error).toHaveBeenCalledWith('pathspec nope did not match'));
    });

    it('branch indicator updates from the adopted echo', async () => {
        renderWith(rich.files, {ahead: 2, behind: 1});
        vi.mocked(await binding('StageFiles')).mockResolvedValue(
            status({ahead: 0, behind: 3}, []) as never
        );
        fireEvent.click(screen.getByTestId('row-changes-work.txt').querySelector('button')!);
        await waitFor(() => expect(screen.getByTestId('staging-branch').textContent).toBe('main ↓3'));
    });

    it('virtualizes: renders a window of a large changeset', () => {
        const many = Array.from({length: 500}, (_, i) => file({path: `f${i}.txt`}));
        renderWith(many);
        expect(screen.getByTestId('row-changes-f0.txt')).toBeInTheDocument();
        expect(screen.queryByTestId('row-changes-f499.txt')).toBeNull();
    });
});
