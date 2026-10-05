import {QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useStatus} from '@/hooks/useStatus';
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

import {getStatus} from '@/lib/api';

function wrapper(client = createQueryClient()) {
    const Inner = ({children}: { children?: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return {Wrapper: Inner, client};
}

const statusPayload = {
    branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0},
    files: [{xy: '.M', path: 'a.txt', staged: false, unstaged: true, change: 'modified'}]
};

describe('useStatus', () => {
    beforeEach(() => {
        vi.mocked(getStatus).mockReset();
        useRepoStore.getState().closeRepo();
    });

    it('does not fetch while no repository is open', async () => {
        const {Wrapper} = wrapper();
        const {result} = renderHook(() => useStatus(), {wrapper: Wrapper});

        await waitFor(() => expect(result.current.isPending).toBe(true));
        expect(getStatus).not.toHaveBeenCalled();
    });

    it('fetches and keys the cache on the open repo path', async () => {
        vi.mocked(getStatus).mockResolvedValue(statusPayload as never);
        useRepoStore.getState().openRepo('/repo/one');
        const {Wrapper, client} = wrapper();

        const {result} = renderHook(() => useStatus(), {wrapper: Wrapper});
        await waitFor(() => expect(result.current.isSuccess).toBe(true));

        expect(result.current.data?.files[0].path).toBe('a.txt');
        expect(client.getQueryData(queryKeys.status('/repo/one'))).toBeTruthy();
    });

    it('surfaces the typed error without throwing', async () => {
        vi.mocked(getStatus).mockRejectedValue({code: 'command_failed', message: 'boom'});
        useRepoStore.getState().openRepo('/repo/one');
        const {Wrapper} = wrapper();

        const {result} = renderHook(() => useStatus(), {wrapper: Wrapper});
        await waitFor(() => expect(result.current.isError).toBe(true));
        expect((result.current.error as {message: string}).message).toBeTruthy();
    });
});
