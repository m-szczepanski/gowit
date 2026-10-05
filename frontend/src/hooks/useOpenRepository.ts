import {useCallback} from 'react';
import {OpenFolder, OpenRepository} from '../../wailsjs/go/main/App';
import {useRepoStore} from '@/stores/repo';

export type OpenOutcome =
    | {status: 'ok'}
    | {status: 'cancelled'}
    | {status: 'error'; message: string};

/**
 * The two-step open flow (issue #13): pick a folder (or reuse a recents
 * entry), then validate and bind it in Go and move the UI out of the empty
 * state. Callers own how outcomes render: the empty state shows errors
 * inline, the header raises a toast.
 */
export function useOpenRepository() {
    const open = useCallback(async (path: string): Promise<OpenOutcome> => {
        try {
            const res = await OpenRepository(path);
            if (res.code) {
                return {status: 'error', message: res.message};
            }
            useRepoStore.getState().openRepo(path);
            return {status: 'ok'};
        } catch (err) {
            return {status: 'error', message: String(err)};
        }
    }, []);

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
