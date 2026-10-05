import {useMutation, useQueryClient} from '@tanstack/react-query';
import {stageAll, stageFiles, type StatusResponse, unstageAll, unstageFiles} from '@/lib/api';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

type RepoSlot = {repoPath: string | null};


/**
 * Staging mutations (#17). The backend answers every mutation with the
 * fresh StatusResponse, so success replaces the status cache with
 * authoritative data - the frontend never re-derives git index transitions
 * the way an optimistic flip would have to. The repo path is captured when
 * the mutation starts (onMutate context), so an echo belongs to the repo
 * it ran against even if the user opened another repo mid-call. On failure
 * the index may be partially changed (StageAll with a bad path aborts per
 * git semantics is not guaranteed), so only error settles invalidate; the
 * watcher's status:changed covers quiet external changes.
 */
function useStatusMutation<TVars>(mutator: (vars: TVars) => Promise<StatusResponse>) {
    const queryClient = useQueryClient();

    return useMutation<StatusResponse, unknown, TVars, RepoSlot>({
        // the arrow drops TanStack's second mutationFn argument (context);
        // the Wails bindings take exactly the variables
        mutationFn: (vars: TVars) => mutator(vars),
        onMutate: () => ({repoPath: useRepoStore.getState().repoPath}),
        // onMutate always supplies the context, so the cast skips a
        // dead undefined branch
        onSuccess: (res, _vars, ctx) => {
            const {repoPath} = ctx as RepoSlot;
            if (repoPath !== null) {
                queryClient.setQueryData(queryKeys.status(repoPath), res);
            }
        },
        onError: (_error, _vars, ctx) => {
            const {repoPath} = ctx as RepoSlot;
            if (repoPath !== null) {
                void queryClient.invalidateQueries({queryKey: queryKeys.status(repoPath)});
            }
        }
    });
}

export function useStageFiles() {
    return useStatusMutation(stageFiles);
}

export function useUnstageFiles() {
    return useStatusMutation(unstageFiles);
}

export function useStageAll() {
    return useStatusMutation<void>(() => stageAll());
}

export function useUnstageAll() {
    return useStatusMutation<void>(() => unstageAll());
}
