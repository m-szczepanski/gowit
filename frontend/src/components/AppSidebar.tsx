import {useRepoStore} from '@/stores/repo';

type Region = {
    title: string;
    testId: string;
    openHint: string;
    closedHint: string;
};

const regions: Region[] = [
    {title: 'Branches', testId: 'branches-placeholder', openHint: 'No branches loaded', closedHint: 'Open a repository to list branches'},
    {title: 'Remotes', testId: 'remotes-placeholder', openHint: 'No remotes loaded', closedHint: 'Open a repository to list remotes'},
    {title: 'Stashes', testId: 'stashes-placeholder', openHint: 'No stashes', closedHint: 'Open a repository to list stashes'}
];

function SidebarSection({region, isOpen}: {region: Region; isOpen: boolean}) {
    return (
        <section aria-labelledby={`${region.testId}-heading`}>
            <h2 id={`${region.testId}-heading`} className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
                {region.title}
            </h2>
            <p className="pt-2 text-sm text-muted-foreground" data-testid={region.testId}>
                {isOpen ? region.openHint : region.closedHint}
            </p>
        </section>
    );
}

export function AppSidebar() {
    const isOpen = useRepoStore((s) => s.isOpen);

    return (
        <aside aria-label="Repository panels" className="flex h-full flex-col gap-6 overflow-y-auto border-r border-sidebar-border bg-sidebar p-3 text-sidebar-foreground">
            {regions.map((region) => (
                <SidebarSection key={region.testId} region={region} isOpen={isOpen} />
            ))}
        </aside>
    );
}
