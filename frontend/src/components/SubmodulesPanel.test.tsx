import {QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {toast} from 'sonner';
import {SubmodulesPanel} from '@/components/SubmodulesPanel';
import {createQueryClient} from '@/lib/queryClient';
import {useRepoStore} from '@/stores/repo';

vi.mock('sonner', () => ({
    toast: {error: vi.fn(), success: vi.fn()}
}));

vi.mock('../../wailsjs/go/main/App', () => ({
    GetSubmodules: vi.fn(),
    SubmoduleInitUpdate: vi.fn(),
    SubmoduleUpdate: vi.fn(),
    SubmoduleDeinit: vi.fn(),
    SubmoduleRemove: vi.fn(),
    SubmoduleAdd: vi.fn(),
    OpenSubmodule: vi.fn()
}));

type BindingName = 'GetSubmodules' | 'SubmoduleInitUpdate' | 'SubmoduleUpdate' | 'SubmoduleDeinit' |
    'SubmoduleRemove' | 'SubmoduleAdd' | 'OpenSubmodule';

async function binding(name: BindingName) {
    const mod = await import('../../wailsjs/go/main/App');
    return mod[name] as ReturnType<typeof vi.fn>;
}

const echo = {
    code: '', message: '', path: '/repo/one',
    branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0}, files: []
};

function listResponse() {
    return {
        code: '',
        message: '',
        path: '/repo/one',
        submodules: [
            {path: 'sub1', sha: '0123456789abcdef0123456789abcdef01234567', describe: 'heads/main', state: 'ok'},
            {path: 'sub2', sha: 'aabbccddeeff00112233445566778899aabbccdd', describe: '', state: 'uninitialized'}
        ]
    };
}

async function renderPanel() {
    for (const name of ['SubmoduleInitUpdate', 'SubmoduleUpdate', 'SubmoduleDeinit', 'SubmoduleRemove',
        'SubmoduleAdd', 'OpenSubmodule'] as BindingName[]) {
        vi.mocked(await binding(name)).mockResolvedValue(echo as never);
    }
    const client = createQueryClient();
    const Wrapper = ({children}: { children?: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return render(<SubmodulesPanel/>, {wrapper: Wrapper});
}

describe('SubmodulesPanel', () => {
    beforeEach(async () => {
        vi.mocked(await binding('GetSubmodules')).mockReset();
        vi.mocked(toast.error).mockClear();
        useRepoStore.getState().openRepo('/repo/one');
    });

    it('lists rows with state-dependent actions', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        await renderPanel();

        expect(await screen.findByText('sub1')).toBeTruthy();
        expect(screen.getByText('01234567 (heads/main)')).toBeTruthy();
        expect(screen.getByText('aabbccdd')).toBeTruthy();
        expect(screen.getByText('uninitialized')).toBeTruthy();
        expect(screen.getByRole('button', {name: 'Update'})).toBeTruthy();
        expect(screen.getByRole('button', {name: 'Init'})).toBeTruthy();
        expect(screen.queryAllByRole('button', {name: 'Deinit'})).toHaveLength(1);
        expect(screen.getAllByRole('button', {name: 'Remove'})).toHaveLength(2);
    });

    it('shows the empty state when nothing is registered', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue({code: '', message: '', path: '/r', submodules: []} as never);
        await renderPanel();
        expect(await screen.findByText('No submodules registered.')).toBeTruthy();
    });

    it('surfaces a failed list read', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue({
            code: 'command_failed', message: 'git exploded', path: '/repo/one', submodules: []
        } as never);
        await renderPanel();
        expect(await screen.findByText('Could not read submodules')).toBeTruthy();
    });

    it('updates a single row without confirmation', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        const update = vi.mocked(await binding('SubmoduleUpdate'));
        await renderPanel();
        await screen.findByText('sub1');

        fireEvent.click(screen.getByRole('button', {name: 'Update'}));
        await waitFor(() => expect(update).toHaveBeenCalledWith('sub1'));
    });

    it('asks confirmation before deinit and then calls the backend', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        const deinit = vi.mocked(await binding('SubmoduleDeinit'));
        await renderPanel();
        await screen.findByText('sub1');

        fireEvent.click(screen.getByRole('button', {name: 'Deinit'}));
        expect(await screen.findByText('Deinitialize submodule')).toBeTruthy();
        const buttons = screen.getAllByRole('button', {name: 'Deinit'});
        fireEvent.click(buttons[buttons.length - 1]!);
        await waitFor(() => expect(deinit).toHaveBeenCalledWith('sub1'));
    });

    it('requires confirmation before removal and then calls the backend', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        await renderPanel();
        await screen.findByText('sub1');

        const remove = vi.mocked(await binding('SubmoduleRemove'));
        fireEvent.click(screen.getAllByRole('button', {name: 'Remove'})[1]!);
        expect(await screen.findByText('Remove submodule')).toBeTruthy();
        const dialogButtons = screen.getAllByRole('button', {name: 'Remove'});
        fireEvent.click(dialogButtons[dialogButtons.length - 1]!);
        await waitFor(() => expect(remove).toHaveBeenCalledWith('sub2'));
    });

    it('closes the confirmation without deleting on cancel', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        await renderPanel();
        await screen.findByText('sub1');

        fireEvent.click(screen.getAllByRole('button', {name: 'Remove'})[0]!);
        await screen.findByText('Remove submodule');
        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));
        await waitFor(() => expect(screen.queryByText('Remove submodule')).toBeNull());
        expect(await binding('SubmoduleRemove')).not.toHaveBeenCalled();
    });

    it('escape closes the confirmation without deleting', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        const remove = vi.mocked(await binding('SubmoduleRemove'));
        await renderPanel();
        await screen.findByText('sub1');

        fireEvent.click(screen.getAllByRole('button', {name: 'Remove'})[0]!);
        await screen.findByText('Remove submodule');
        fireEvent.keyDown(document, {key: 'Escape'});
        await waitFor(() => expect(screen.queryByText('Remove submodule')).toBeNull());
        expect(remove).not.toHaveBeenCalled();
    });

    it('deep-links into a submodule through the store', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        await renderPanel();
        vi.mocked(await binding('OpenSubmodule')).mockResolvedValue({code: '', message: '', path: '/repo/one/sub1'} as never);
        await screen.findByText('sub1');

        fireEvent.click(screen.getAllByRole('button', {name: 'Open'})[0]!);
        await waitFor(() => expect(useRepoStore.getState().repoPath).toBe('/repo/one/sub1'));
    });

    it('init-update-all respects the recursive checkbox and add submits both fields', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        await renderPanel();
        await screen.findByText('sub1');

        const initAll = vi.mocked(await binding('SubmoduleInitUpdate'));
        const add = vi.mocked(await binding('SubmoduleAdd'));
        fireEvent.click(screen.getByLabelText('recursive'));
        fireEvent.click(screen.getByRole('button', {name: 'Init & update all'}));
        await waitFor(() => expect(initAll).toHaveBeenCalledWith(true));
        fireEvent.click(screen.getByLabelText('recursive'));
        fireEvent.click(screen.getByRole('button', {name: 'Init & update all'}));
        await waitFor(() => expect(initAll).toHaveBeenLastCalledWith(false));

        fireEvent.change(screen.getByLabelText('Submodule URL'), {target: {value: 'https://host/o/r.git'}});
        fireEvent.change(screen.getByLabelText('Submodule path'), {target: {value: 'sub3'}});
        fireEvent.click(screen.getByRole('button', {name: 'Add'}));
        await waitFor(() => expect(add).toHaveBeenCalledWith('https://host/o/r.git', 'sub3'));
        expect(screen.getByLabelText('Submodule URL').getAttribute('value')).toBe('');
    });

    it('toasts the typed failure from a mutation', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(listResponse() as never);
        await renderPanel();
        vi.mocked(await binding('SubmoduleUpdate')).mockResolvedValue({
            code: 'command_failed', message: 'git said no', path: '/repo/one', branch: {}, files: []
        } as never);
        await screen.findByText('sub1');

        fireEvent.click(screen.getByRole('button', {name: 'Update'}));
        await waitFor(() => expect(toast.error).toHaveBeenCalledWith('git said no'));
    });
});
