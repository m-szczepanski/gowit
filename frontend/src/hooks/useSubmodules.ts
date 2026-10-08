import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {
    getSubmodules,
    openSubmodule,
    submoduleAdd,
    submoduleDeinit,
    submoduleInitUpdate,
    submoduleRemove,
    submoduleUpdate,
    type StatusResponse
} from '@/lib/api';
import {queryKeys} from '@/lib/queryKeys';
import {useRepoStore} from '@/stores/repo';

/**
 * Submodules of the open repo (#76). Keyed by repo path like everything
 * else; repo:status-changed invalidates the key from the event hook, and
 * each mutation below additionally invalidates it because its echoes
 * carry only status data, not the list.
 */
export function useSubmodules() {
    const repoPath = useRepoStore((s) => s.repoPath);

    return useQuery({
        queryKey: queryKeys.submodules(repoPath ?? ''),
        queryFn: getSubmodules,
        enabled: repoPath !== null
    });
}

type RepoSlot = {repoPath: string | null};

function useSubmoduleMutation<TVars>(mutator: (vars: TVars) => Promise<StatusResponse>) {
    const queryClient = useQueryClient();

    return useMutation<StatusResponse, unknown, TVars, RepoSlot>({
        mutationFn: (vars: TVars) => mutator(vars),
        onMutate: () => ({repoPath: useRepoStore.getState().repoPath}),
        onSuccess: (res, _vars, ctx) => {
            const {repoPath} = ctx as RepoSlot;
            if (repoPath !== null) {
                queryClient.setQueryData(queryKeys.status(repoPath), res);
                void queryClient.invalidateQueries({queryKey: queryKeys.submodules(repoPath)});
            }
        },
        onError: (_error, _vars, ctx) => {
            const {repoPath} = ctx as RepoSlot;
            if (repoPath !== null) {
                void queryClient.invalidateQueries({queryKey: queryKeys.status(repoPath)});
                void queryClient.invalidateQueries({queryKey: queryKeys.submodules(repoPath)});
            }
        }
    });
}

export function useSubmoduleInitUpdate() {
    return useSubmoduleMutation(submoduleInitUpdate);
}

export function useSubmoduleUpdate() {
    return useSubmoduleMutation(submoduleUpdate);
}

export function useSubmoduleDeinit() {
    return useSubmoduleMutation(submoduleDeinit);
}

export function useSubmoduleRemove() {
    return useSubmoduleMutation(submoduleRemove);
}

export function useSubmoduleAdd() {
    return useSubmoduleMutation<{url: string; path: string}>(({url, path}) => submoduleAdd(url, path));
}

/**
 * Deep link into a submodule: Go validates registration and init state and
 * switches its own slot; the store follows the resolved path it returns,
 * exactly like the open-repository flow does.
 */
export function useOpenSubmodule() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (path: string) => openSubmodule(path),
        onSuccess: (res) => {
            useRepoStore.getState().openRepo(res.path);
            void queryClient.invalidateQueries({queryKey: queryKeys.recentRepos()});
            void queryClient.invalidateQueries({queryKey: queryKeys.status(res.path)});
            void queryClient.invalidateQueries({queryKey: queryKeys.submodules(res.path)});
        }
    });
}
