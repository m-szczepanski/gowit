import {create} from 'zustand';

type RepoState = {
    repoPath: string | null;
    isOpen: boolean;
    openRepo: (path: string) => void;
    closeRepo: () => void;
};

export const useRepoStore = create<RepoState>((set) => ({
    repoPath: null,
    isOpen: false,
    openRepo: (path) => set({repoPath: path, isOpen: true}),
    closeRepo: () => set({repoPath: null, isOpen: false})
}));
