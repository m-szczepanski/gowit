import {useCallback, useEffect, useState} from 'react';
import {GetRecentRepos} from '../../wailsjs/go/main/App';
import type {config} from '../../wailsjs/go/models';

export type RecentRepo = config.RecentRepo;

export function useRecentRepos() {
    const [repos, setRepos] = useState<RecentRepo[]>([]);

    const refresh = useCallback(
        () =>
            GetRecentRepos()
                .then(setRepos)
                .catch(() => {
                    // no recents shown is the only failure mode worth
                    // hiding; #14 will surface config errors
                }),
        []
    );

    useEffect(() => {
        void refresh();
    }, [refresh]);

    return {repos, refresh};
}
