/**
 * Single convention for cache keys, one factory per data source.
 * Keys always start with the data name, then the repo path the call ran
 * against, then the query's own parameters. Adding a new backend read
 * means adding one factory here, so invalidation stays enumerable.
 */
export const queryKeys = {
    status: (repoPath: string) => ['status', repoPath] as const,
    log: (repoPath: string, params: {branch?: string; limit?: number} = {}) =>
        ['log', repoPath, params] as const,
    diff: (repoPath: string, commitHash: string, filePath: string | null = null) =>
        ['diff', repoPath, commitHash, filePath] as const
};
