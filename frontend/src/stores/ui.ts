import {create} from 'zustand';

export type PanelId = 'status' | 'submodules' | 'history' | 'graph';

type UiState = {
    activePanel: PanelId;
    selectedCommit: string | null;
    selectedFile: string | null;
    setActivePanel: (panel: PanelId) => void;
    selectCommit: (hash: string | null) => void;
    selectFile: (path: string | null) => void;
};

export const useUiStore = create<UiState>((set) => ({
    activePanel: 'status',
    selectedCommit: null,
    selectedFile: null,
    setActivePanel: (panel) => set({activePanel: panel}),
    // Clearing the file selection keeps the diff panel from showing a file
    // that does not exist in the newly selected commit.
    selectCommit: (hash) => set({selectedCommit: hash, selectedFile: null}),
    selectFile: (path) => set({selectedFile: path})
}));
