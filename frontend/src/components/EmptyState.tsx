import {useState} from 'react';
import {FolderOpen} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {AddRecentRepo, OpenFolder} from '../../wailsjs/go/main/App';
import {useRecentRepos} from '@/hooks/useRecentRepos';
import {useRepoStore} from '@/stores/repo';

export function EmptyState() {
    const [error, setError] = useState('');
    const openRepo = useRepoStore((s) => s.openRepo);
    const {repos} = useRecentRepos();

    const handleOpen = async () => {
        setError('');
        try {
            const result = await OpenFolder();
            if (result.code) {
                setError(result.message);
            } else if (result.path) {
                openRepo(result.path);
            }
        } catch (err) {
            setError(String(err));
        }
    };

    const reopen = (path: string) => {
        openRepo(path);
        void AddRecentRepo(path);
    };

    return (
        <div className="flex h-full flex-col items-center justify-center gap-4 bg-background" data-testid="empty-state">
            <FolderOpen className="text-muted-foreground" size={48} aria-hidden="true" />
            <div className="text-center">
                <h2 className="text-lg font-semibold">No repository open</h2>
                <p className="text-sm text-muted-foreground">Open a folder containing a git repository to get started.</p>
            </div>
            <Button onClick={handleOpen}>Open Folder</Button>
            {repos.length > 0 && (
                <ul className="w-full max-w-md space-y-1 px-4" data-testid="recent-repos">
                    {repos.map((repo) => (
                        <li key={repo.path}>
                            <button
                                type="button"
                                className="w-full truncate rounded px-2 py-1 text-left font-mono text-xs text-muted-foreground hover:bg-accent hover:text-accent-foreground"
                                onClick={() => reopen(repo.path)}
                            >
                                {repo.path}
                            </button>
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
