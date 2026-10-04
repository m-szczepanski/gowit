import {create} from 'zustand';

export type PanelId = 'status' | 'history' | 'diff';

type UiState = {
    activePanel: PanelId;
    selectedCommit: string | null;
    selectedFile: string | null;
    setActivePanel: (panel: PanelId) => void;
    selectCommit: (hash: string | null) => void;
    selectFile: (path: string | null) => void;
};

/**
 * Pure view state: which panel is visible and what is selected in it.
 * A new commit selection clears the file selection, so the diff panel
 * never shows a file from the previous commit.
 */
export const useUiStore = create<UiState>((set) => ({
    activePanel: 'status',
    selectedCommit: null,
    selectedFile: null,
    setActivePanel: (panel) => set({activePanel: panel}),
    selectCommit: (hash) => set({selectedCommit: hash, selectedFile: null}),
    selectFile: (path) => set({selectedFile: path})
}));
