import type {QueryClient, QueryKey} from '@tanstack/react-query';

/**
 * Minimal view of a Wails-style event bus. Matches runtime.EventsOn exactly:
 * subscribing returns an unsubscribe closure (runtime.EventsOff drops ALL
 * listeners for an event name, so it is useless for scoped detach).
 * Wiring the real bus is therefore just:
 *
 *   bindInvalidatingEvents(client, {on: runtime.EventsOn}, [
 *     {event: 'repo:changed', queryKey: queryKeys.status(path)}
 *   ]);
 */
export type EventSource = {
    on(event: string, callback: () => void): () => void;
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
    const unsubscribes = bindings.map(({event, queryKey}) =>
        source.on(event, () => client.invalidateQueries({queryKey}))
    );

    return () => {
        for (const unsubscribe of unsubscribes) {
            unsubscribe();
        }
    };
}
