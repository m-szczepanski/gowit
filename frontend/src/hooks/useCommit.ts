import {useMutation, useQueryClient} from '@tanstack/react-query';
import {commit} from '@/lib/api';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

/**
 * Commit mutation (#21). Settled, success or failure, invalidates the
 * status cache and every cached history page of the repo the call started
 * against: a rejected hook can still have changed the index. Status also
 * arrives via the Go worker's repo:status-changed push, and TanStack
 * dedupes the overlapping fetches. The log cache has no pusher, so this
 * invalidation is its only refresher even before #25 adds the query.
 */
export function useCommit() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: ({message, amend}: {message: string; amend?: boolean}) => commit(message, amend),
        // same convention as the staging mutations: the repo is captured
        // when the call starts, so a switch mid-commit cannot leave the
        // rewritten repo's cache stale
        onMutate: () => ({repoPath: useRepoStore.getState().repoPath}),
        onSettled: (_data, _error, _vars, ctx) => {
            const repoPath = ctx?.repoPath;
            if (repoPath) {
                void queryClient.invalidateQueries({queryKey: queryKeys.status(repoPath)});
                void queryClient.invalidateQueries({queryKey: queryKeys.logPrefix(repoPath)});
            }
        }
    });
}
