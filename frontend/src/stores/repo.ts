import {create} from 'zustand';

type RepoState = {
    repoPath: string | null;
    isOpen: boolean;
    openRepo: (path: string) => void;
    closeRepo: () => void;
};

/**
 * Which repository the app is currently pointed at.
 * Set by the open-repo flow; the file watcher and status tasks subscribe
 * to changes through useRepoStore.subscribe.
 */
export const useRepoStore = create<RepoState>((set) => ({
    repoPath: null,
    isOpen: false,
    openRepo: (path) => set({repoPath: path, isOpen: true}),
    closeRepo: () => set({repoPath: null, isOpen: false})
}));
