import {useQuery} from '@tanstack/react-query';
import {queryKeys} from '@/lib/queryKeys';
import {getStatus} from '@/lib/api';
import {useRepoStore} from '@/stores/repo';

/**
 * Working-dir state of the open repository (#17). Keyed by the resolved
 * repo path (#6 convention); queries are paused while no repo is open.
 */
export function useStatus() {
    const repoPath = useRepoStore((s) => s.repoPath);

    return useQuery({
        queryKey: queryKeys.status(repoPath ?? ''),
        queryFn: getStatus,
        enabled: repoPath !== null
    });
}
