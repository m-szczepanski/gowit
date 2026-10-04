import type {ReactNode} from 'react';
import {QueryClientProvider, useQuery} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import {EventsOn} from '../../wailsjs/runtime/runtime';
import {createQueryClient} from './queryClient';
import {queryKeys} from './queryKeys';
import {bindInvalidatingEvents, type EventSource} from './wailsEvents';

type FakeBus = EventSource & {emit(event: string): void};

function createFakeBus(): FakeBus {
    const listeners = new Map<string, Set<() => void>>();
    return {
        on(event, callback) {
            if (!listeners.has(event)) listeners.set(event, new Set());
            listeners.get(event)!.add(callback);
            return () => listeners.get(event)?.delete(callback);
        },
        emit(event) {
            listeners.get(event)?.forEach((cb) => cb());
        }
    };
}

describe('bindInvalidatingEvents', () => {
    it('refetches the mapped query on event and not after cleanup', async () => {
        const client = createQueryClient();
        const wrapper = ({children}: {children: ReactNode}) => (
            <QueryClientProvider client={client}>{children}</QueryClientProvider>
        );
        const queryFn = vi.fn().mockResolvedValue('files');
        renderHook(() => useQuery({queryKey: queryKeys.status('/repo'), queryFn}), {wrapper});
        await waitFor(() => expect(queryFn).toHaveBeenCalledTimes(1));

        const bus = createFakeBus();
        const cleanup = bindInvalidatingEvents(client, bus, [
            {event: 'repo:changed', queryKey: queryKeys.status('/repo')}
        ]);

        bus.emit('repo:changed');
        await waitFor(() => expect(queryFn).toHaveBeenCalledTimes(2));

        cleanup();
        bus.emit('repo:changed');
        expect(queryFn).toHaveBeenCalledTimes(2);
    });

    it('accepts the real wails runtime EventsOn as the source', () => {
        const source: EventSource = {on: EventsOn};
        expect(typeof source.on).toBe('function');
    });
});
