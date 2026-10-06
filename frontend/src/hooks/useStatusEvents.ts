import {useEffect} from 'react';
import {useQueryClient} from '@tanstack/react-query';
import {EventsOn} from '../../wailsjs/runtime/runtime';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';
import type {main} from '../../wailsjs/go/models';

/**
 * Live status refresh (#19): the Go watcher debounces filesystem churn and
 * each quiet burst emits repo:status-changed with a fresh StatusResponse
 * payload, so the UI adopts the snapshot outright instead of refetching -
 * that replace-by-payload is also the coalescing: duplicate or out-of-order
 * signals just rewrite the same cache key with equally fresh data.
 * When the payload is unusable (transport noise, failed backend read, or a
 * snapshot of a repo the user already switched away from) the handler falls
 * back to invalidating the key, which TanStack deduplicates per active
 * query. Payloads are plain JSON over the bridge, structurally identical to
 * the models.StatusResponse the query stores.
 */
export function useStatusEvents() {
    const queryClient = useQueryClient();
    const repoPath = useRepoStore((s) => s.repoPath);

    useEffect(() => {
        if (repoPath === null) {
            return;
        }
        return EventsOn('repo:status-changed', (payload?: main.StatusResponse) => {
            if (payload && !payload.code && payload.path === repoPath) {
                queryClient.setQueryData(queryKeys.status(repoPath), payload);
                return;
            }
            void queryClient.invalidateQueries({queryKey: queryKeys.status(repoPath)});
        });
    }, [queryClient, repoPath]);
}
