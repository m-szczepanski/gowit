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

describe('useCommit', () => {
    beforeEach(() => {
        useRepoStore.getState().openRepo('/repo/one');
    });

    it('sends message and amend flag to the binding', async () => {
        vi.mocked(await binding()).mockResolvedValue({code: '', message: ''});
        const client = createQueryClient();
        const Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await result.current.mutateAsync({message: 'feat: x', amend: true});

        expect(await binding()).toHaveBeenCalledWith('feat: x', true);
    });

    it('invalidates the status cache after success', async () => {
        vi.mocked(await binding()).mockResolvedValue({code: '', message: ''});
        const client = createQueryClient();
        const invalidate = vi.spyOn(client, 'invalidateQueries');
        const Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await result.current.mutateAsync({message: 'm'});

        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
    });

    it('rejects with the structured ApiError and still refreshes', async () => {
        vi.mocked(await binding()).mockResolvedValue({
            code: 'commit_rejected', message: 'pre-commit said no'
        });
        const client = createQueryClient();
        const invalidate = vi.spyOn(client, 'invalidateQueries');
        const Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await expect(result.current.mutateAsync({message: 'm'})).rejects.toMatchObject({code: 'commit_rejected'});
        await waitFor(() => expect(result.current.isError).toBe(true));
        expect(invalidate).toHaveBeenCalled();
    });

    it('skips invalidation when no repository is open', async () => {
        useRepoStore.getState().closeRepo();
        vi.mocked(await binding()).mockResolvedValue({code: '', message: ''});
        const client = createQueryClient();
        const invalidate = vi.spyOn(client, 'invalidateQueries');
        const Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );

        const {result} = renderHook(() => useCommit(), {wrapper: Wrapper});
        await result.current.mutateAsync({message: 'm'});

        expect(invalidate).not.toHaveBeenCalled();
    });
});
