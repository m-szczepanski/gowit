import {beforeEach, describe, expect, it} from 'vitest';
import {useUiStore} from './ui';

describe('useUiStore', () => {
    beforeEach(() => {
        useUiStore.getState().setActivePanel('status');
        useUiStore.getState().selectCommit(null);
        useUiStore.getState().selectFile(null);
    });

    it('shows the status panel by default', () => {
        expect(useUiStore.getState().activePanel).toBe('status');
    });

    it('switches the active panel', () => {
        useUiStore.getState().setActivePanel('history');
        expect(useUiStore.getState().activePanel).toBe('history');
    });

    it('selecting a commit clears the previously selected file', () => {
        useUiStore.getState().selectFile('src/main.go');
        useUiStore.getState().selectCommit('a1b2c3d');
        expect(useUiStore.getState()).toMatchObject({
            selectedCommit: 'a1b2c3d',
            selectedFile: null
        });
    });

    it('selecting a file does not touch the commit selection', () => {
        useUiStore.getState().selectCommit('a1b2c3d');
        useUiStore.getState().selectFile('internal/git/status.go');
        expect(useUiStore.getState().selectedCommit).toBe('a1b2c3d');
    });
});
