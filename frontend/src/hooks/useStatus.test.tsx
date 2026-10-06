import {QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useStatus} from '@/hooks/useStatus';
import {createQueryClient} from '@/lib/queryClient';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

vi.mock('../../wailsjs/go/main/App', () => ({
    GetStatus: vi.fn()
}));

async function binding() {
    const mod = await import('../../wailsjs/go/main/App');
    return mod.GetStatus as ReturnType<typeof vi.fn>;
}

function wrapper(client = createQueryClient()) {
    const Inner = ({children}: { children?: ReactNode }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return {Wrapper: Inner, client};
}

const okPayload = {
    code: '',
    message: '',
    branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0},
    files: [{xy: '.M', path: 'a.txt', staged: false, unstaged: true, change: 'modified'}]
};

describe('useStatus', () => {
    beforeEach(async () => {
        vi.mocked(await binding()).mockReset();
        useRepoStore.getState().closeRepo();
    });

    it('does not fetch while no repository is open', async () => {
        const {Wrapper} = wrapper();
        const {result} = renderHook(() => useStatus(), {wrapper: Wrapper});

        await waitFor(() => expect(result.current.isPending).toBe(true));
        expect(await binding()).not.toHaveBeenCalled();
    });

    it('fetches parsed data and keys the cache on the open repo path', async () => {
        vi.mocked(await binding()).mockResolvedValue(okPayload as never);
        useRepoStore.getState().openRepo('/repo/one');
        const {Wrapper, client} = wrapper();

        const {result} = renderHook(() => useStatus(), {wrapper: Wrapper});
        await waitFor(() => expect(result.current.isSuccess).toBe(true));

        expect(result.current.data?.files[0].path).toBe('a.txt');
        expect(client.getQueryData(queryKeys.status('/repo/one'))).toBeTruthy();
    });

    it('surfaces the typed backend error as query error, not a throw', async () => {
        vi.mocked(await binding()).mockResolvedValue({
            code: 'no_repo', message: 'no repository open', branch: {}, files: []
        } as never);
        useRepoStore.getState().openRepo('/repo/one');
        const {Wrapper} = wrapper();

        const {result} = renderHook(() => useStatus(), {wrapper: Wrapper});
        await waitFor(() => expect(result.current.isError).toBe(true));
        expect((result.current.error as {code?: string}).code).toBe('no_repo');
    });
});
