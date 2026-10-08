import {QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {
    useOpenSubmodule,
    useSubmoduleInitUpdate,
    useSubmoduleRemove,
    useSubmodules
} from '@/hooks/useSubmodules';
import {createQueryClient} from '@/lib/queryClient';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

vi.mock('../../wailsjs/go/main/App', () => ({
    GetSubmodules: vi.fn(),
    SubmoduleInitUpdate: vi.fn(),
    SubmoduleUpdate: vi.fn(),
    SubmoduleDeinit: vi.fn(),
    SubmoduleRemove: vi.fn(),
    SubmoduleAdd: vi.fn(),
    OpenSubmodule: vi.fn()
}));

type BindingName = 'GetSubmodules' | 'SubmoduleInitUpdate' | 'SubmoduleRemove' | 'OpenSubmodule';

async function binding(name: BindingName) {
    const mod = await import('../../wailsjs/go/main/App');
    return mod[name] as ReturnType<typeof vi.fn>;
}

const sub = (path: string, state: string) => ({
    path, sha: '0123456789abcdef0123456789abcdef01234567', describe: 'heads/main', state
});

const list = (...subs: ReturnType<typeof sub>[]) => ({code: '', message: '', path: '/repo/one', submodules: subs});

const echo = {
    code: '',
    message: '',
    path: '/repo/one',
    branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0},
    files: []
};

function setup() {
    const client = createQueryClient();
    const Wrapper = ({children}: { children?: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return {client, Wrapper};
}

describe('useSubmodules', () => {
    beforeEach(async () => {
        for (const name of ['GetSubmodules', 'SubmoduleInitUpdate', 'SubmoduleRemove', 'OpenSubmodule'] as BindingName[]) {
            vi.mocked(await binding(name)).mockReset();
        }
        useRepoStore.getState().openRepo('/repo/one');
    });

    it('fetches the list keyed by the open repo', async () => {
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(list(sub('sub1', 'ok')) as never);
        const {Wrapper} = setup();
        const {result} = renderHook(() => useSubmodules(), {wrapper: Wrapper});
        await waitFor(() => expect(result.current.isSuccess).toBe(true));
        expect(result.current.data?.submodules[0].path).toBe('sub1');
    });

    it('replaces the status cache with the echo and invalidates the list', async () => {
        vi.mocked(await binding('SubmoduleInitUpdate')).mockResolvedValue(echo as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useSubmoduleInitUpdate(), {wrapper: Wrapper});
        await result.current.mutateAsync(true);

        expect(await binding('SubmoduleInitUpdate')).toHaveBeenCalledWith(true);
        expect(client.getQueryData(queryKeys.status('/repo/one'))).toEqual(echo);
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.submodules('/repo/one')});
    });

    it('invalidates both keys when a mutation fails with a typed code', async () => {
        vi.mocked(await binding('SubmoduleRemove')).mockResolvedValue({
            code: 'validation_failed', message: 'not a registered submodule: nope', path: '/repo/one', branch: {}, files: []
        } as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useSubmoduleRemove(), {wrapper: Wrapper});
        await expect(result.current.mutateAsync('nope')).rejects.toThrow('not a registered submodule');
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.submodules('/repo/one')});
    });

    it('stays inert without an open repo', async () => {
        useRepoStore.getState().closeRepo();
        vi.mocked(await binding('GetSubmodules')).mockResolvedValue(list() as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const query = renderHook(() => useSubmodules(), {wrapper: Wrapper});
        expect(query.result.current.fetchStatus).toBe('idle');
        expect(await binding('GetSubmodules')).not.toHaveBeenCalled();

        vi.mocked(await binding('SubmoduleInitUpdate')).mockResolvedValue(echo as never);
        const ok = renderHook(() => useSubmoduleInitUpdate(), {wrapper: Wrapper});
        await ok.result.current.mutateAsync(true);
        expect(client.getQueryData(queryKeys.status(''))).toBeUndefined();

        vi.mocked(await binding('SubmoduleRemove')).mockResolvedValue({
            code: 'no_repo', message: 'none', path: '', branch: {}, files: []
        } as never);
        const bad = renderHook(() => useSubmoduleRemove(), {wrapper: Wrapper});
        await expect(bad.result.current.mutateAsync('sub1')).rejects.toThrow('none');
        expect(invalidate).not.toHaveBeenCalled();
    });

    it('switches the repo store when a deep link opens a submodule', async () => {
        vi.mocked(await binding('OpenSubmodule')).mockResolvedValue({code: '', message: '', path: '/repo/one/sub1'} as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useOpenSubmodule(), {wrapper: Wrapper});
        await result.current.mutateAsync('sub1');

        expect(useRepoStore.getState().repoPath).toBe('/repo/one/sub1');
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.submodules('/repo/one/sub1')});
    });
});
