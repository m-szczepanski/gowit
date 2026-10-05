import {QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useStatusEvents} from '@/hooks/useStatusEvents';
import {createQueryClient} from '@/lib/queryClient';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

type Listener = () => void;

const bus = new Map<string, Set<Listener>>();
let subscribeCalls = 0;

vi.mock('../../wailsjs/runtime/runtime', () => ({
    EventsOn: (event: string, cb: Listener) => {
        subscribeCalls++;
        if (!bus.has(event)) {
            bus.set(event, new Set());
        }
        bus.get(event)!.add(cb);
        return () => {
            bus.get(event)!.delete(cb);
        };
    }
}));

function fire(event: string) {
    for (const cb of bus.get(event) ?? []) {
        cb();
    }
}

describe('useStatusEvents', () => {
    let client: ReturnType<typeof createQueryClient>;
    let invalidate: ReturnType<typeof vi.spyOn>;
    let Wrapper: ({children}: { children?: ReactNode }) => ReactNode;

    beforeEach(() => {
        bus.clear();
        subscribeCalls = 0;
        client = createQueryClient();
        invalidate = vi.spyOn(client, 'invalidateQueries');
        Wrapper = ({children}: {children?: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );
        useRepoStore.getState().closeRepo();
    });

    it('subscribes on open repo and invalidates the status key on status:changed', async () => {
        useRepoStore.getState().openRepo('/repo/one');
        renderHook(() => useStatusEvents(), {wrapper: Wrapper});
        await waitFor(() => expect(subscribeCalls).toBe(1));

        fire('status:changed');
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
    });

    it('subscribes nothing while closed, and resubscribes on repo switch', async () => {
        const {rerender, unmount} = renderHook(() => useStatusEvents(), {wrapper: Wrapper});
        await waitFor(() => expect(subscribeCalls).toBe(0));
        expect(invalidate).not.toHaveBeenCalled();

        useRepoStore.getState().openRepo('/repo/two');
        rerender();
        await waitFor(() => expect(subscribeCalls).toBe(1));

        fire('status:changed');
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/two')});

        unmount();
        fire('status:changed');
        expect(invalidate).toHaveBeenCalledTimes(1);
    });

    it('only fires for its own event name', async () => {
        useRepoStore.getState().openRepo('/repo/one');
        renderHook(() => useStatusEvents(), {wrapper: Wrapper});
        await waitFor(() => expect(subscribeCalls).toBe(1));

        fire('repo:opened');
        expect(invalidate).not.toHaveBeenCalled();
    });
});
