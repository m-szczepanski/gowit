import {useMutation, useQueryClient} from '@tanstack/react-query';
import {commit} from '@/lib/api';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

/**
 * Commit mutation (#21). Success invalidates the status cache: the Go
 * worker also pushes repo:status-changed, and TanStack dedupes the
 * concurrent fetches, so the invalidation is the belt to that suspenders -
 * a commit that races its own watcher stays consistent. History
 * invalidation joins here when the log query lands (#25).
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
            }
        }
    });
}
