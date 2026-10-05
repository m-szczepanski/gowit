import {useEffect} from 'react';
import {useQueryClient} from '@tanstack/react-query';
import {EventsOn} from '../../wailsjs/runtime/runtime';
import {bindInvalidatingEvents} from '@/lib/wailsEvents';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

/**
 * Bridges backend signals into the status cache (#17): the Go watcher
 * emits status:changed after its debounce, and every mutation or open
 * must refresh the query it keys on the resolved repo path. Listeners are
 * detached on repo switch and unmount via the binder's cleanup.
 */
export function useStatusEvents() {
    const queryClient = useQueryClient();
    const repoPath = useRepoStore((s) => s.repoPath);

    useEffect(() => {
        if (repoPath === null) {
            return;
        }
        return bindInvalidatingEvents(queryClient, {on: EventsOn}, [
            {event: 'status:changed', queryKey: queryKeys.status(repoPath)}
        ]);
    }, [queryClient, repoPath]);
}
