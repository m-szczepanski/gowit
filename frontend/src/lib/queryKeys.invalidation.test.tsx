import type {ReactNode} from 'react';
import {QueryClientProvider, useQuery} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import {createQueryClient} from './queryClient';
import {queryKeys} from './queryKeys';

describe('repo-scoped invalidation via key factories', () => {
    it('refetches only the invalidated repo while both are observed', async () => {
        const client = createQueryClient();
        const wrapper = ({children}: {children: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );

        const queryFnA = vi.fn().mockResolvedValue('files a');
        const queryFnB = vi.fn().mockResolvedValue('files b');

        renderHook(() => useQuery({queryKey: queryKeys.status('/repo-a'), queryFn: queryFnA}), {wrapper});
        renderHook(() => useQuery({queryKey: queryKeys.status('/repo-b'), queryFn: queryFnB}), {wrapper});

        await waitFor(() => expect(queryFnA).toHaveBeenCalledTimes(1));
        await waitFor(() => expect(queryFnB).toHaveBeenCalledTimes(1));

        client.invalidateQueries({queryKey: queryKeys.status('/repo-a')});

        await waitFor(() => expect(queryFnA).toHaveBeenCalledTimes(2));
        expect(queryFnB).toHaveBeenCalledTimes(1);
    });
});
