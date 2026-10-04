import {describe, expect, it} from 'vitest';
import {queryKeys} from './queryKeys';

describe('queryKeys', () => {
    it('keys status by data name and repo path', () => {
        expect(queryKeys.status('/home/user/repo')).toEqual(['status', '/home/user/repo']);
    });

    it('keys log by repo path and its query params', () => {
        expect(queryKeys.log('/repo')).toEqual(['log', '/repo', {}]);
        expect(queryKeys.log('/repo', {branch: 'main', limit: 100})).toEqual([
            'log',
            '/repo',
            {branch: 'main', limit: 100}
        ]);
    });

    it('keys diff by repo path, commit and optional file', () => {
        expect(queryKeys.diff('/repo', 'a1b2c3d')).toEqual(['diff', '/repo', 'a1b2c3d', null]);
        expect(queryKeys.diff('/repo', 'a1b2c3d', 'main.go')).toEqual([
            'diff',
            '/repo',
            'a1b2c3d',
            'main.go'
        ]);
    });
});
