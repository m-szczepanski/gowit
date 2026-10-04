import {useRepoStore} from '@/stores/repo';

export function StatusBar() {
    const repoPath = useRepoStore((s) => s.repoPath);

    return (
        <footer aria-label="Status bar" className="flex items-center gap-4 border-t border-border bg-background px-3 py-1 font-mono text-xs text-muted-foreground">
            <span data-testid="status-branch">{repoPath ? 'branch: n/a' : 'no repository'}</span>
            <span data-testid="ahead-behind">↑ — ↓ —</span>
            <span aria-live="polite" className="ml-auto" data-testid="operation-progress" />
        </footer>
    );
}
