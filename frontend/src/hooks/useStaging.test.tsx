import {QueryClientProvider} from '@tanstack/react-query';
import {act, renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useStageAll, useStageFiles, useUnstageAll, useUnstageFiles} from '@/hooks/useStaging';
import {createQueryClient} from '@/lib/queryClient';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

vi.mock('../../wailsjs/go/main/App', () => ({
    StageFiles: vi.fn(),
    UnstageFiles: vi.fn(),
    StageAll: vi.fn(),
    UnstageAll: vi.fn()
}));

type BindingName = 'StageFiles' | 'UnstageFiles' | 'StageAll' | 'UnstageAll';

async function binding(name: BindingName) {
    const mod = await import('../../wailsjs/go/main/App');
    return mod[name] as ReturnType<typeof vi.fn>;
}

const echo = (label: string) => ({
    code: '',
    message: '',
    branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0},
    files: [{xy: 'M.', path: `${label}.txt`, staged: true, unstaged: false, change: 'modified'}]
});

function setup() {
    const client = createQueryClient();
    const Wrapper = ({children}: { children?: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return {client, Wrapper};
}

describe('useStaging mutations', () => {
    beforeEach(async () => {
        for (const name of ['StageFiles', 'UnstageFiles', 'StageAll', 'UnstageAll'] as BindingName[]) {
            vi.mocked(await binding(name)).mockReset();
        }
        useRepoStore.getState().openRepo('/repo/one');
    });

    it('replaces the status cache with the backend echo and does not refetch', async () => {
        vi.mocked(await binding('StageFiles')).mockResolvedValue(echo('staged') as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useStageFiles(), {wrapper: Wrapper});
        await result.current.mutateAsync(['a.txt']);

        expect(await binding('StageFiles')).toHaveBeenCalledWith(['a.txt']);
        const cached = client.getQueryData(queryKeys.status('/repo/one')) as {
            files: {path: string}[]
        } | undefined;
        expect(cached?.files[0].path).toBe('staged.txt');
        expect(invalidate).not.toHaveBeenCalled();
    });

    it('routes unstage, stageAll and unstageAll to their bindings', async () => {
        vi.mocked(await binding('UnstageFiles')).mockResolvedValue(echo('u') as never);
        vi.mocked(await binding('StageAll')).mockResolvedValue(echo('s') as never);
        vi.mocked(await binding('UnstageAll')).mockResolvedValue(echo('sa') as never);
        const {Wrapper} = setup();

        const unstage = renderHook(() => useUnstageFiles(), {wrapper: Wrapper});
        await unstage.result.current.mutateAsync(['b.txt']);
        expect(await binding('UnstageFiles')).toHaveBeenCalledWith(['b.txt']);

        const stage = renderHook(() => useStageAll(), {wrapper: Wrapper});
        await stage.result.current.mutateAsync();
        expect(await binding('StageAll')).toHaveBeenCalled();

        const unstageEverything = renderHook(() => useUnstageAll(), {wrapper: Wrapper});
        await unstageEverything.result.current.mutateAsync();
        expect(await binding('UnstageAll')).toHaveBeenCalled();
    });

    it('rejects with ApiError and refetches when the backend failed', async () => {
        vi.mocked(await binding('StageFiles')).mockResolvedValue({
            code: 'command_failed',
            message: 'pathspec nope did not match',
            branch: {},
            files: []
        } as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useStageFiles(), {wrapper: Wrapper});
        await expect(result.current.mutateAsync(['nope'])).rejects.toMatchObject({code: 'command_failed'});
        await waitFor(() => expect(result.current.isError).toBe(true));
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
    });

    it('keys the echo on the repo the call started against, not the current one', async () => {
        vi.mocked(await binding('StageFiles')).mockResolvedValue(echo('late') as never);
        const {client, Wrapper} = setup();

        const {result} = renderHook(() => useStageFiles(), {wrapper: Wrapper});
        // onMutate captures the slot synchronously; the switch below lands
        // before any promise settles
        const pending = result.current.mutateAsync(['a.txt']);
        useRepoStore.getState().openRepo('/repo/two');
        await pending;

        expect(client.getQueryData(queryKeys.status('/repo/one'))).toBeTruthy();
        expect(client.getQueryData(queryKeys.status('/repo/two'))).toBeUndefined();
    });

    it('skips cache writes when no repository was open at call start', async () => {
        useRepoStore.getState().closeRepo();
        vi.mocked(await binding('StageAll')).mockResolvedValue(echo('s') as never);
        const {client, Wrapper} = setup();
        const setQueryData = vi.spyOn(client, 'setQueryData');

        const {result} = renderHook(() => useStageAll(), {wrapper: Wrapper});
        await result.current.mutateAsync();

        expect(setQueryData).not.toHaveBeenCalled();
    });

    it('skips the error refetch too when no repository was open', async () => {
        useRepoStore.getState().closeRepo();
        vi.mocked(await binding('StageAll')).mockResolvedValue({
            code: 'no_repo', message: 'gone', branch: {}, files: []
        } as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useStageAll(), {wrapper: Wrapper});
        await expect(result.current.mutateAsync()).rejects.toMatchObject({code: 'no_repo'});
        expect(invalidate).not.toHaveBeenCalled();
    });
});
