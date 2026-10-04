import {QueryClient} from '@tanstack/react-query';

/**
 * Query defaults for a desktop app backed by Go calls:
 *
 * - staleTime 30s: results (status, log, diff) are fresh within one user
 *   action; re-render storms do not re-shell git.
 * - gcTime 10min: caches of closed repos fall out of memory eventually.
 * - refetchOnWindowFocus false: this is not a web tab; focus means nothing.
 * - retry false: a Go binding rejecting is a bug or real git error, not a
 *   transient network fault; show it immediately.
 *
 * Cache invalidation strategy (issue #6 convention):
 * 1. User-initiated refresh: call invalidateQueries on the affected key
 *    factory from lib/queryKeys (e.g. queryKeys.status(repoPath)).
 * 2. Repo/file changes detected by the watcher: Wails events are bridged to
 *    invalidations via lib/wailsEvents, never polled by components.
 * 3. After any mutating Go call (commit, stage, checkout): invalidate on
 *    success in the mutation's own module, scoped to the smallest key that
 *    changed (status and log, not the whole cache).
 */
export function createQueryClient() {
    return new QueryClient({
        defaultOptions: {
            queries: {
                staleTime: 30_000,
                gcTime: 600_000,
                refetchOnWindowFocus: false,
                retry: false
            }
        }
    });
}
