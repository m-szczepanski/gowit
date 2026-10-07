import {QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useCommit} from '@/hooks/useCommit';
import {createQueryClient} from '@/lib/queryClient';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

vi.mock('../../wailsjs/go/main/App', () => ({
    Commit: vi.fn()
}));

async function binding() {
    const mod = await import('../../wailsjs/go/main/App');
    return mod.Commit as ReturnType<typeof vi.fn>;
}

function setup() {
    const client = createQueryClient();
    const Wrapper = ({children}: {children?: ReactNode}) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return {client, Wrapper};
}

describe('useCommit', () => {
    beforeEach(() => {
        useRepoStore.getState().openRepo('/repo/one');
    });

    it('sends message and amend flag to the binding', async () => {
        vi.mocked(await binding()).mockResolvedValue({code: '', message: ''});
        const {Wrapper} = setup();

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await result.current.mutateAsync({message: 'feat: x', amend: true});

        expect(await binding()).toHaveBeenCalledWith('feat: x', true);
    });

    it('invalidates the status cache after success', async () => {
        vi.mocked(await binding()).mockResolvedValue({code: '', message: ''});
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await result.current.mutateAsync({message: 'm'});

        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
    });

    it('rejects with the structured ApiError and still refreshes', async () => {
        vi.mocked(await binding()).mockResolvedValue({
            code: 'commit_rejected', message: 'pre-commit said no'
        });
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await expect(result.current.mutateAsync({message: 'm'})).rejects.toMatchObject({code: 'commit_rejected'});
        await waitFor(() => expect(result.current.isError).toBe(true));
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
    });

    it('invalidates the repo the commit started against, not the current one', async () => {
        let release: (v: unknown) => void = () => {
        };
        vi.mocked(await binding()).mockImplementation(
            () => new Promise((resolve) => {
                release = resolve;
            })
        );
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        const pending = result.current.mutateAsync({message: 'm'});
        await waitFor(() => expect(invalidate).not.toHaveBeenCalled());
        useRepoStore.getState().openRepo('/repo/two');
        release({code: '', message: ''});
        await pending;

        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
        expect(invalidate).not.toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/two')});
    });

    it('skips invalidation when no repository was open at call start', async () => {
        useRepoStore.getState().closeRepo();
        vi.mocked(await binding()).mockResolvedValue({code: '', message: ''});
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await result.current.mutateAsync({message: 'm'});

        expect(invalidate).not.toHaveBeenCalled();
    });
});
