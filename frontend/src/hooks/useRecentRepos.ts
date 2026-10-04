import {useEffect, useState} from 'react';
import {GetRecentRepos} from '../../wailsjs/go/main/App';
import type {config} from '../../wailsjs/go/models';

export function useRecentRepos() {
    const [repos, setRepos] = useState<config.RecentRepo[]>([]);

    useEffect(() => {
        GetRecentRepos()
            .then(setRepos)
            .catch(() => {
                // an empty recents list is the only visible failure mode;
                // issue #14 will surface config errors
            });
    }, []);

    return repos;
}
