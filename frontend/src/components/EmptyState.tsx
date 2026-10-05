import {useState} from 'react';
import {FolderOpen} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {useOpenRepository, type OpenOutcome} from '@/hooks/useOpenRepository';
import {useRecentRepos} from '@/hooks/useRecentRepos';

export function EmptyState() {
    const [error, setError] = useState('');
    const repos = useRecentRepos().repos;
    const {open, browse} = useOpenRepository();

    const run = async (outcome: Promise<OpenOutcome>) => {
        setError('');
        const result = await outcome;
        if (result.status === 'error') {
            setError(result.message ?? 'failed to open repository');
        }
    };

    return (
        <div className="flex h-full flex-col items-center justify-center gap-4 bg-background" data-testid="empty-state">
            <FolderOpen className="text-muted-foreground" size={48} aria-hidden="true" />
            <div className="text-center">
                <h2 className="text-lg font-semibold">No repository open</h2>
                <p className="text-sm text-muted-foreground">Open a folder containing a git repository to get started.</p>
            </div>
            <Button onClick={() => void run(browse())}>Open Folder</Button>
            {repos.length > 0 && (
                <ul aria-label="Recent repositories" className="w-full max-w-md space-y-1 px-4" data-testid="recent-repos">
                    {repos.map((repo) => (
                        <li key={repo.path}>
                            <Button variant="ghost" size="sm" className="w-full justify-start" onClick={() => void run(open(repo.path))}>
                                <span className="min-w-0 flex-1 truncate font-mono text-left text-xs">{repo.path}</span>
                            </Button>
                        </li>
                    ))}
                </ul>
            )}
            {error && (
                <p className="text-sm text-destructive" data-testid="open-error">
                    {error}
                </p>
            )}
        </div>
    );
}
