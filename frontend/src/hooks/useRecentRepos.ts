import {useQuery} from '@tanstack/react-query';
import {GetRecentRepos} from '../../wailsjs/go/main/App';
import {queryKeys} from '@/lib/queryKeys';

/**
 * Shared cached list (issue #12/#13 convention: Go call = queryFn). One
 * query keeps every consumer - header menu and empty state - consistent;
 * useOpenRepository invalidates the key after a successful open.
 */
export function useRecentRepos() {
    const query = useQuery({
        queryKey: queryKeys.recentRepos(),
        queryFn: () => GetRecentRepos()
    });

    return {repos: query.data ?? []};
}
