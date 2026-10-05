import {QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useStageAll, useStageFiles, useUnstageAll, useUnstageFiles} from '@/hooks/useStaging';
import {stageAll, stageFiles, unstageAll, unstageFiles} from '@/lib/api';
import {createQueryClient} from '@/lib/queryClient';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

vi.mock('@/lib/api', () => ({
    getStatus: vi.fn(),
    stageFiles: vi.fn(),
    unstageFiles: vi.fn(),
    stageAll: vi.fn(),
    unstageAll: vi.fn()
}));

const echoPayload = (label: string) => ({
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
    beforeEach(() => {
        for (const fn of [stageFiles, unstageFiles, stageAll, unstageAll]) {
            vi.mocked(fn).mockReset();
        }
        useRepoStore.getState().openRepo('/repo/one');
    });

    it('replaces the status cache with the backend echo and invalidates on settle', async () => {
        vi.mocked(stageFiles).mockResolvedValue(echoPayload('staged') as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useStageFiles(), {wrapper: Wrapper});
        await result.current.mutateAsync(['a.txt']);

        expect(stageFiles).toHaveBeenCalledWith(['a.txt']);
        const cached = client.getQueryData(queryKeys.status('/repo/one')) as {
            files: {path: string}[]
        } | undefined;
        expect(cached?.files[0].path).toBe('staged.txt');
        await waitFor(() => expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')}));
    });

    it('routes unstage, stageAll and unstageAll to their api calls', async () => {
        vi.mocked(unstageFiles).mockResolvedValue(echoPayload('u') as never);
        vi.mocked(stageAll).mockResolvedValue(echoPayload('s') as never);
        vi.mocked(unstageAll).mockResolvedValue(echoPayload('sa') as never);
        const {Wrapper} = setup();

        const unstage = renderHook(() => useUnstageFiles(), {wrapper: Wrapper});
        await unstage.result.current.mutateAsync(['b.txt']);
        expect(unstageFiles).toHaveBeenCalledWith(['b.txt']);

        const stage = renderHook(() => useStageAll(), {wrapper: Wrapper});
        await stage.result.current.mutateAsync([]);
        expect(stageAll).toHaveBeenCalled();

        const unstageEverything = renderHook(() => useUnstageAll(), {wrapper: Wrapper});
        await unstageEverything.result.current.mutateAsync([]);
        expect(unstageAll).toHaveBeenCalled();
    });

    it('keeps the error state visible to callers without throwing', async () => {
        vi.mocked(stageFiles).mockRejectedValue(Object.assign(new Error('pathspec nope'), {code: 'command_failed'}));
        const {Wrapper} = setup();

        const {result} = renderHook(() => useStageFiles(), {wrapper: Wrapper});
        await expect(result.current.mutateAsync(['nope'])).rejects.toThrow('pathspec nope');
        await waitFor(() => expect(result.current.isError).toBe(true));
        expect((result.current.error as {code?: string}).code).toBe('command_failed');
    });

    it('skips cache writes when no repository is open', async () => {
        useRepoStore.getState().closeRepo();
        vi.mocked(stageAll).mockResolvedValue(echoPayload('s') as never);
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useStageAll(), {wrapper: Wrapper});
        await result.current.mutateAsync([]);

        expect(client.getQueryData(queryKeys.status(''))).toBeUndefined();
        expect(invalidate).not.toHaveBeenCalled();
    });
});
