import {Button} from '@/components/ui/button';
import {useRepoStore} from '@/stores/repo';

function repoName(repoPath: string | null): string {
    if (!repoPath) {
        return 'no repository';
    }
    return repoPath.split(/[\\/]/).filter(Boolean).pop() ?? repoPath;
}

type Props = {
    sidebarOpen: boolean;
    onToggleSidebar: () => void;
};

export function AppHeader({sidebarOpen, onToggleSidebar}: Props) {
    const repoPath = useRepoStore((s) => s.repoPath);

    return (
        <header aria-label="Main toolbar" className="flex items-center gap-2 border-b border-border bg-background px-3 py-2">
            <Button variant="ghost" size="sm" aria-label="Toggle sidebar" aria-expanded={sidebarOpen} onClick={onToggleSidebar}>
                Sidebar
            </Button>
            <h1 className="truncate text-sm font-semibold">{repoName(repoPath)}</h1>
            <span className="text-muted-foreground text-xs" data-testid="branch-indicator">
                {repoPath ? 'no branch info' : '—'}
            </span>
            <div className="ml-auto flex gap-2">
                <Button size="sm" variant="outline" disabled>
                    Fetch
                </Button>
                <Button size="sm" variant="outline" disabled>
                    Pull
                </Button>
                <Button size="sm" variant="outline" disabled>
                    Push
                </Button>
            </div>
        </header>
    );
}
