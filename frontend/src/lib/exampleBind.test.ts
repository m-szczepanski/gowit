import {describe, expect, it, vi} from 'vitest';
import {createQueryClient} from './queryClient';
import {exampleBindOptions} from './exampleBind';

vi.mock('../../wailsjs/go/main/App', () => ({
    ExampleBind: vi.fn()
}));

describe('Go call as queryFn (example bind)', () => {
    it('caches the binding result within staleTime', async () => {
        const {ExampleBind} = await import('../../wailsjs/go/main/App');
        const call = ExampleBind as ReturnType<typeof vi.fn>;
        call.mockResolvedValue('gowit backend is reachable');

        const client = createQueryClient();
        const first = await client.fetchQuery(exampleBindOptions);
        const second = await client.fetchQuery(exampleBindOptions);

        expect(first).toBe('gowit backend is reachable');
        expect(second).toBe('gowit backend is reachable');
        expect(call).toHaveBeenCalledTimes(1);
    });

    it('surfaces binding rejection without retrying', async () => {
        const {ExampleBind} = await import('../../wailsjs/go/main/App');
        const call = ExampleBind as ReturnType<typeof vi.fn>;
        call.mockRejectedValue(new Error('go: binding missing'));

        const client = createQueryClient();
        await expect(client.fetchQuery(exampleBindOptions)).rejects.toThrow('go: binding missing');
        expect(call).toHaveBeenCalledTimes(1);
    });
});
