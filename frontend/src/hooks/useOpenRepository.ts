import {useCallback} from 'react';
import {useQueryClient} from '@tanstack/react-query';
import {OpenFolder, OpenRepository} from '../../wailsjs/go/main/App';
import {useRepoStore} from '@/stores/repo';
import {queryKeys} from '@/lib/queryKeys';

export type OpenOutcome =
    | {status: 'ok'; path: string}
    | {status: 'cancelled'}
    | {status: 'error'; message: string};

/**
 * The two-step open flow (issue #13): pick a folder (or reuse a recents
 * entry), then validate and bind it in Go and move the UI out of the empty
 * state. The store takes the resolved work-tree path returned by Go, which
 * differs from the requested path for subdirectories or symlinked roots.
 * Callers own how outcomes render: the empty state shows errors inline, the
 * header raises a toast.
 */
export function useOpenRepository() {
    const queryClient = useQueryClient();

    const open = useCallback(
        async (path: string): Promise<OpenOutcome> => {
            try {
                const res = await OpenRepository(path);
                if (res.code) {
                    return {status: 'error', message: res.message};
                }
                useRepoStore.getState().openRepo(res.path);
                queryClient.invalidateQueries({queryKey: queryKeys.recentRepos()});
                return {status: 'ok', path: res.path};
            } catch (err) {
                return {status: 'error', message: String(err)};
            }
        },
        [queryClient]
    );

    const browse = useCallback(async (): Promise<OpenOutcome> => {
        try {
            const picked = await OpenFolder();
            if (picked.code) {
                return {status: 'error', message: picked.message};
            }
            if (!picked.path) {
                return {status: 'cancelled'};
            }
            return await open(picked.path);
        } catch (err) {
            return {status: 'error', message: String(err)};
        }
    }, [open]);

    return {open, browse};
}
