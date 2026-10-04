import {useRepoStore} from '@/stores/repo';

export function AppSidebar() {
    const isOpen = useRepoStore((s) => s.isOpen);

    return (
        <aside aria-label="Repository panels" className="flex h-full flex-col gap-6 overflow-y-auto border-r border-sidebar-border bg-sidebar p-3 text-sidebar-foreground">
            <section aria-labelledby="branches-heading">
                <h2 id="branches-heading" className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
                    Branches
                </h2>
                <p className="pt-2 text-sm text-muted-foreground" data-testid="branches-placeholder">
                    {isOpen ? 'No branches loaded' : 'Open a repository to list branches'}
                </p>
            </section>
            <section aria-labelledby="remotes-heading">
                <h2 id="remotes-heading" className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
                    Remotes
                </h2>
                <p className="pt-2 text-sm text-muted-foreground" data-testid="remotes-placeholder">
                    {isOpen ? 'No remotes loaded' : 'Open a repository to list remotes'}
                </p>
            </section>
            <section aria-labelledby="stashes-heading">
                <h2 id="stashes-heading" className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
                    Stashes
                </h2>
                <p className="pt-2 text-sm text-muted-foreground" data-testid="stashes-placeholder">
                    {isOpen ? 'No stashes' : 'Open a repository to list stashes'}
                </p>
            </section>
        </aside>
    );
}
