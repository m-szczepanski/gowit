import type {QueryClient, QueryKey} from '@tanstack/react-query';

/**
 * Minimal view of a Wails-style event bus (runtime.EventsOn/EventsOff).
 * Kept as an injected dependency so this module is testable without the
 * Wails runtime and can be fed the real bus when the watcher lands:
 *
 *   bindInvalidatingEvents(client, {
 *     on: runtime.EventsOn,
 *     off: runtime.EventsOff
 *   }, [{event: 'repo:changed', queryKey: queryKeys.status(path)}]);
 */
export type EventSource = {
    on(event: string, callback: () => void): void;
    off(event: string, callback: () => void): void;
};

export type EventInvalidation = {
    event: string;
    queryKey: QueryKey;
};

/**
 * Bridges backend events to cache invalidation: when an event fires, the
 * mapped query key is invalidated and active observers refetch. Returns a
 * cleanup function that detaches all listeners (used on repo close/shutdown).
 */
export function bindInvalidatingEvents(
    client: QueryClient,
    source: EventSource,
    bindings: EventInvalidation[]
): () => void {
    const handlers = bindings.map(({event, queryKey}) => {
        const handler = () => client.invalidateQueries({queryKey});
        source.on(event, handler);
        return {event, handler};
    });

    return () => {
        for (const {event, handler} of handlers) {
            source.off(event, handler);
        }
    };
}
