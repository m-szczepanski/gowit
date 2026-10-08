import {QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useStatusEvents} from '@/hooks/useStatusEvents';
import {createQueryClient} from '@/lib/queryClient';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

type Handler = (...data: unknown[]) => void;

const listeners = new Map<string, Set<Handler>>();
let subscribeCount = 0;

vi.mock('../../wailsjs/runtime/runtime', () => ({
    EventsOn: (event: string, cb: Handler) => {
        subscribeCount++;
        if (!listeners.has(event)) {
            listeners.set(event, new Set());
        }
        listeners.get(event)!.add(cb);
        return () => {
            listeners.get(event)!.delete(cb);
        };
    }
}));

function fire(event: string, ...data: unknown[]) {
    for (const cb of listeners.get(event) ?? []) {
        cb(...data);
    }
}

const snapshot = (path: string) => ({
    code: '',
    message: '',
    path,
    branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0},
    files: [{xy: '.M', path: 'a.txt', staged: false, unstaged: true, change: 'modified'}]
});

function setup() {
    const client = createQueryClient();
    const Wrapper = ({children}: {children?: ReactNode}) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
    return {client, Wrapper};
}

describe('useStatusEvents', () => {
    beforeEach(() => {
        listeners.clear();
        subscribeCount = 0;
        useRepoStore.getState().closeRepo();
    });

    it('adopts a matching payload into the status cache without refetching', async () => {
        useRepoStore.getState().openRepo('/repo/one');
        const {client, Wrapper} = setup();
        const invalidate = vi.spyOn(client, 'invalidateQueries');
        const setQueryData = vi.spyOn(client, 'setQueryData');

        renderHook(() => useStatusEvents(), {wrapper: Wrapper});
        await waitFor(() => expect(subscribeCount).toBe(1));

        fire('repo:status-changed', snapshot('/repo/one'));
        expect(setQueryData).toHaveBeenCalledWith(queryKeys.status('/repo/one'), snapshot('/repo/one'));
        expect(invalidate).not.toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.submodules('/repo/one')});
    });

    it('invalidates instead of adopting when the payload is stale or broken', async () => {
        useRepoStore.getState().openRepo('/repo/one');
        const {client, Wrapper} = setup();
        const setQueryData = vi.spyOn(client, 'setQueryData');

        const invalidate = vi.spyOn(client, 'invalidateQueries');
        renderHook(() => useStatusEvents(), {wrapper: Wrapper});
        await waitFor(() => expect(subscribeCount).toBe(1));

        fire('repo:status-changed', snapshot('/repo/other'));
        fire('repo:status-changed', {code: 'no_repo', message: 'gone', path: '', branch: {}, files: []});
        fire('repo:status-changed');
        expect(setQueryData).not.toHaveBeenCalled();
        expect(invalidate).toHaveBeenCalledTimes(6);
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.status('/repo/one')});
        expect(invalidate).toHaveBeenCalledWith({queryKey: queryKeys.submodules('/repo/one')});
    });

    it('subscribes only while open, retargets on switch, detaches on unmount', async () => {
        const {client, Wrapper} = setup();
        const setQueryData = vi.spyOn(client, 'setQueryData');
        const {rerender, unmount} = renderHook(() => useStatusEvents(), {wrapper: Wrapper});
        await waitFor(() => expect(subscribeCount).toBe(0));

        useRepoStore.getState().openRepo('/repo/one');
        rerender();
        await waitFor(() => expect(subscribeCount).toBe(1));
        expect(listeners.get('repo:status-changed')!.size).toBe(1);

        useRepoStore.getState().openRepo('/repo/two');
        rerender();
        await waitFor(() => expect(subscribeCount).toBe(2));
        expect(listeners.get('repo:status-changed')!.size).toBe(1);

        unmount();
        expect(listeners.get('repo:status-changed')!.size).toBe(0);
        fire('repo:status-changed', snapshot('/repo/two'));
        expect(setQueryData).not.toHaveBeenCalled();
    });
});
