/**
 * Single convention for cache keys, one factory per data source plus
 * explicit prefix factories for repo-wide invalidation. Keys always start
 * with the data name, then the repo path the call ran against, then the
 * query's own parameters. Adding a new backend read means adding one
 * factory here, so invalidation stays enumerable.
 */
export const queryKeys = {
    recentRepos: () => ['recentRepos'] as const,
    status: (repoPath: string) => ['status', repoPath] as const,
    log: (repoPath: string, params: {branch?: string; limit?: number} = {}) =>
        ['log', repoPath, params] as const,
    // prefix of every ['log', repoPath, params] variant, for mutations
    // that dirty all cached pages at once (a new commit rewrites history
    // everywhere)
    logPrefix: (repoPath: string) => ['log', repoPath] as const,
    diff: (repoPath: string, commitHash: string, filePath: string | null = null) =>
        ['diff', repoPath, commitHash, filePath] as const
};
