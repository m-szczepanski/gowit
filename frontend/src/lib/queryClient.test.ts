import {describe, expect, it} from 'vitest';
import {createQueryClient} from './queryClient';

describe('createQueryClient', () => {
    it('applies the desktop query defaults', () => {
        const client = createQueryClient();
        const defaults = client.getDefaultOptions().queries;
        expect(defaults?.staleTime).toBe(30_000);
        expect(defaults?.gcTime).toBe(600_000);
        expect(defaults?.refetchOnWindowFocus).toBe(false);
        expect(defaults?.retry).toBe(false);
    });
});
