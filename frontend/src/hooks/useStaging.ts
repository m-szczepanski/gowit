import {useMutation, useQueryClient} from '@tanstack/react-query';
import {stageAll, stageFiles, type StatusResponse, unstageAll, unstageFiles} from '@/lib/api';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

/**
 * Staging mutations (#17). The backend answers every mutation with the
 * fresh StatusResponse, so success replaces the status cache with
 * authoritative data - the frontend never re-derives git index transitions
 * the way an optimistic flip would have to. onSettled invalidates as a
 * safety net, and ApiError from the mutationFn feeds toast/inline display.
 */
function useStagingMutation(mutator: (paths: string[]) => Promise<StatusResponse>) {
    const queryClient = useQueryClient();
    const repoPath = useRepoStore((s) => s.repoPath);

    return useMutation({
        // the arrow drops the second argument TanStack passes (mutation
        // context); the Wails bindings take exactly the path list
        mutationFn: (paths: string[]) => mutator(paths),
        onSuccess: (res) => {
            if (repoPath !== null) {
                queryClient.setQueryData(queryKeys.status(repoPath), res);
            }
        },
        onSettled: () => {
            if (repoPath !== null) {
                void queryClient.invalidateQueries({queryKey: queryKeys.status(repoPath)});
            }
        }
    });
}

export function useStageFiles() {
    return useStagingMutation(stageFiles);
}

export function useUnstageFiles() {
    return useStagingMutation(unstageFiles);
}

export function useStageAll() {
    return useStagingMutation(() => stageAll());
}

export function useUnstageAll() {
    return useStagingMutation(() => unstageAll());
}
