import {toast} from 'sonner';
import {Button} from '@/components/ui/button';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger
} from '@/components/ui/dropdown-menu';
import {useOpenRepository, type OpenOutcome} from '@/hooks/useOpenRepository';
import {useRecentRepos} from '@/hooks/useRecentRepos';
import {SIDEBAR_PANEL_ID} from '@/hooks/useSidebarCollapse';
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
    const {repos} = useRecentRepos();
    const {open, browse} = useOpenRepository();

    const handle = async (outcome: Promise<OpenOutcome>) => {
        const result = await outcome;
        if (result.status === 'error') {
            toast.error(result.message);
        }
    };

    return (
        <header aria-label="Main toolbar" className="flex shrink-0 items-center gap-2 border-b border-border bg-background px-3 py-2">
            <Button variant="ghost" size="sm" aria-label="Toggle sidebar" aria-expanded={sidebarOpen} aria-controls={SIDEBAR_PANEL_ID} onClick={onToggleSidebar}>
                Sidebar
            </Button>
            <h1 className="truncate text-sm font-semibold">{repoName(repoPath)}</h1>
            <span className="text-muted-foreground text-xs" data-testid="branch-indicator">
                {repoPath ? 'no branch info' : '—'}
            </span>
            <div className="ml-auto flex gap-2">
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button size="sm" variant="outline">
                            Open
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-72">
                        <DropdownMenuLabel>Recent repositories</DropdownMenuLabel>
                        {repos.map((repo) => (
                            <DropdownMenuItem key={repo.path} onSelect={() => void handle(open(repo.path))}>
                                <span className="min-w-0 flex-1 truncate font-mono text-xs">{repo.path}</span>
                            </DropdownMenuItem>
                        ))}
                        {repos.length > 0 && <DropdownMenuSeparator />}
                        <DropdownMenuItem onSelect={() => void handle(browse())}>
                            Browse folders…
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
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
