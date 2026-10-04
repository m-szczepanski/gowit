import {beforeEach, describe, expect, it} from 'vitest';
import {useRepoStore} from './repo';

describe('useRepoStore', () => {
    beforeEach(() => {
        useRepoStore.getState().closeRepo();
    });

    it('starts with no repo open', () => {
        const {repoPath, isOpen} = useRepoStore.getState();
        expect(repoPath).toBeNull();
        expect(isOpen).toBe(false);
    });

    it('openRepo records the path and marks the repo open', () => {
        useRepoStore.getState().openRepo('/home/user/project');
        expect(useRepoStore.getState()).toMatchObject({
            repoPath: '/home/user/project',
            isOpen: true
        });
    });

    it('closeRepo resets to the closed state', () => {
        useRepoStore.getState().openRepo('/tmp/whatever');
        useRepoStore.getState().closeRepo();
        expect(useRepoStore.getState()).toMatchObject({repoPath: null, isOpen: false});
    });
});
