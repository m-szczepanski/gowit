import {queryOptions} from '@tanstack/react-query';
import {ExampleBind} from '../../wailsjs/go/main/App';
import {queryKeys} from './queryKeys';

/**
 * The "Go call = queryFn" pattern (issue #6):
 *
 * A bound Wails method is just an async function, so every Go read goes
 * into TanStack Query as the queryFn, with its key from lib/queryKeys:
 *
 *   export const statusOptions = (repoPath: string) =>
 *     queryOptions({
 *       queryKey: queryKeys.status(repoPath),
 *       queryFn: () => GetStatus(repoPath)
 *     });
 *
 * Components then call useQuery(statusOptions(path)) and get caching,
 * deduplication and ref-on-invalidate for free. Mutating calls stay as
 * direct binding invocations inside useMutation, which invalidates the
 * keys the mutation changed (see lib/queryClient.ts strategy block).
 *
 * ExampleBind below is the placeholder round-trip from issue #4; it will
 * be replaced by real status/log queries as those tasks land.
 */
export const exampleBindOptions = queryOptions({
    queryKey: queryKeys.exampleBind(),
    queryFn: () => ExampleBind()
});
