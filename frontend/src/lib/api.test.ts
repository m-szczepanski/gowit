import {beforeEach, describe, expect, it, vi} from 'vitest';
import {ApiError, getStatus, stageAll, stageFiles, unstageAll, unstageFiles} from '@/lib/api';

vi.mock('../../wailsjs/go/main/App', () => ({
    GetStatus: vi.fn(),
    StageFiles: vi.fn(),
    UnstageFiles: vi.fn(),
    StageAll: vi.fn(),
    UnstageAll: vi.fn()
}));

async function binding(name: 'GetStatus' | 'StageFiles' | 'UnstageFiles' | 'StageAll' | 'UnstageAll') {
    const mod = await import('../../wailsjs/go/main/App');
    return mod[name] as ReturnType<typeof vi.fn>;
}

const okResponse = {
    code: '',
    message: '',
    branch: {head: 'main', oid: 'abc', detached: false, upstream: '', ahead: 0, behind: 0},
    files: [{xy: '.M', path: 'a.txt', untracked: false, ignored: false, conflict: false, staged: false, unstaged: true, change: 'modified'}]
};

describe('api', () => {
    beforeEach(async () => {
        vi.mocked(await binding('GetStatus')).mockResolvedValue(okResponse as never);
        for (const name of ['StageFiles', 'UnstageFiles', 'StageAll', 'UnstageAll'] as const) {
            vi.mocked(await binding(name)).mockResolvedValue(okResponse as never);
        }
    });

    it('getStatus passes the parsed payload through', async () => {
        const res = await getStatus();
        expect(res.files[0].change).toBe('modified');
        expect(res.branch.head).toBe('main');
    });

    it('maps a backend code to ApiError instead of returning the payload', async () => {
        vi.mocked(await binding('GetStatus')).mockResolvedValue({
            code: 'no_repo', message: 'no repository open', branch: {}, files: []
        } as never);

        const err = await getStatus().catch((e: unknown) => e);
        expect(err).toBeInstanceOf(ApiError);
        expect((err as ApiError).code).toBe('no_repo');
    });

    it('every staging call forwards its path list to the binding', async () => {
        await stageFiles(['a.txt', 'b c.txt']);
        expect(await binding('StageFiles')).toHaveBeenCalledWith(['a.txt', 'b c.txt']);

        await unstageFiles(['a.txt']);
        expect(await binding('UnstageFiles')).toHaveBeenCalledWith(['a.txt']);

        await stageAll();
        await unstageAll();
        expect(await binding('StageAll')).toHaveBeenCalled();
        expect(await binding('UnstageAll')).toHaveBeenCalled();
    });

    it('staging errors surface as ApiError with the git code', async () => {
        vi.mocked(await binding('StageFiles')).mockResolvedValue({
            code: 'command_failed', message: 'pathspec nope did not match', branch: {}, files: []
        } as never);

        const err = await stageFiles(['nope']).catch((e: unknown) => e);
        expect((err as ApiError).code).toBe('command_failed');
    });

    it('transport rejections pass through unchanged', async () => {
        vi.mocked(await binding('UnstageAll')).mockRejectedValue(new Error('bindings not ready'));
        await expect(unstageAll()).rejects.toThrow('bindings not ready');
    });
});
