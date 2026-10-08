import {useState} from 'react';
import {toast} from 'sonner';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Checkbox} from '@/components/ui/checkbox';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle
} from '@/components/ui/dialog';
import {Input} from '@/components/ui/input';
import {
    useOpenSubmodule,
    useSubmoduleAdd,
    useSubmoduleDeinit,
    useSubmoduleInitUpdate,
    useSubmoduleRemove,
    useSubmoduleUpdate,
    useSubmodules
} from '@/hooks/useSubmodules';
import type {git} from '../../wailsjs/go/models';

type Submodule = git.Submodule;

function fail(error: unknown) {
    toast.error((error as Error).message);
}

/**
 * Submodules list and lifecycle for the open repo (#76). Add stages the
 * gitlink and .gitmodules like raw git; committing stays in the staging
 * tab. Remove is destructive, so it requires an explicit confirmation.
 */
export function SubmodulesPanel() {
    const submodules = useSubmodules();
    const initUpdateAll = useSubmoduleInitUpdate();
    const updateOne = useSubmoduleUpdate();
    const deinit = useSubmoduleDeinit();
    const remove = useSubmoduleRemove();
    const add = useSubmoduleAdd();
    const open = useOpenSubmodule();
    const busy = initUpdateAll.isPending || updateOne.isPending || deinit.isPending ||
        remove.isPending || add.isPending || open.isPending;

    const [recursive, setRecursive] = useState(false);
    const [url, setUrl] = useState('');
    const [path, setPath] = useState('');
    const [removeTarget, setRemoveTarget] = useState<string | null>(null);

    const items = submodules.data?.submodules ?? [];

    return (
        <div className="flex flex-col gap-3 p-3">
            <div className="flex items-center gap-2">
                <label className="flex items-center gap-2 text-sm text-muted-foreground" htmlFor="submodules-recursive">
                    <Checkbox
                        checked={recursive}
                        id="submodules-recursive"
                        onCheckedChange={(checked) => setRecursive(checked === true)}
                    />
                    recursive
                </label>
                <Button disabled={busy} onClick={() => initUpdateAll.mutate(recursive, {onError: fail})} size="sm"
                      variant="outline">
                    Init &amp; update all
                </Button>
                <span className="ml-auto text-xs text-muted-foreground">
                    {submodules.isFetching ? 'Refreshing...' : ''}
                </span>
            </div>

            {submodules.isError && (
                <p className="text-sm text-destructive">Could not read submodules</p>
            )}
            {!submodules.isError && items.length === 0 && !submodules.isPending && (
                <p className="text-sm text-muted-foreground">No submodules registered.</p>
            )}

            <ul className="flex flex-col gap-2">
                {items.map((s) => (
                    <li className="flex items-center gap-2 rounded-md border p-2" key={s.path}>
                        <Badge variant="secondary">{s.state}</Badge>
                        <span className="font-mono text-sm">{s.path}</span>
                        <span className="text-xs text-muted-foreground">
                            {s.sha.slice(0, 8)}
                            {s.describe ? ` (${s.describe})` : ''}
                        </span>
                        <span className="ml-auto flex gap-1">
                            <Button
                                disabled={busy}
                                onClick={() => updateOne.mutate(s.path, {onError: fail})}
                                size="sm"
                                variant="outline"
                            >
                                {s.state === 'uninitialized' ? 'Init' : 'Update'}
                            </Button>
                            {s.state !== 'uninitialized' && (
                                <Button
                                    disabled={busy}
                                    onClick={() => deinit.mutate(s.path, {onError: fail})}
                                    size="sm"
                                    variant="outline"
                                >
                                    Deinit
                                </Button>
                            )}
                            <Button
                                disabled={busy || s.state === 'uninitialized'}
                                onClick={() => open.mutate(s.path, {onError: fail})}
                                size="sm"
                                variant="outline"
                            >
                                Open
                            </Button>
                            <Button
                                disabled={busy}
                                onClick={() => setRemoveTarget(s.path)}
                                size="sm"
                                variant="outline"
                            >
                                Remove
                            </Button>
                        </span>
                    </li>
                ))}
            </ul>

            <form
                className="flex items-center gap-2"
                onSubmit={(event) => {
                    event.preventDefault();
                    add.mutate(
                        {url, path},
                        {
                            onError: fail,
                            onSuccess: () => {
                                setUrl('');
                                setPath('');
                                toast.success('Submodule added; stage is ready for commit in Status.');
                            }
                        }
                    );
                }}
            >
                <Input
                    aria-label="Submodule URL"
                    className="max-w-60"
                    onChange={(e) => setUrl(e.target.value)}
                    placeholder="https://host/owner/repo.git"
                    value={url}
                />
                <Input
                    aria-label="Submodule path"
                    className="max-w-32"
                    onChange={(e) => setPath(e.target.value)}
                    placeholder="path"
                    value={path}
                />
                <Button disabled={busy || !url || !path} size="sm" type="submit" variant="outline">
                    Add
                </Button>
            </form>

            {removeTarget !== null && (
                <RemoveDialog
                    busy={remove.isPending}
                    onCancel={() => setRemoveTarget(null)}
                    onConfirm={() => remove.mutate(removeTarget, {
                        onError: fail,
                        onSuccess: () => setRemoveTarget(null)
                    })}
                    path={removeTarget}
                />
            )}
        </div>
    );
}

function RemoveDialog({path, busy, onCancel, onConfirm}: {
    path: string;
    busy: boolean;
    onCancel: () => void;
    onConfirm: () => void;
}) {
    return (
        <Dialog onOpenChange={onCancel} open>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Remove submodule</DialogTitle>
                    <DialogDescription>
                        {`Remove "${path}" deinitializes it, deletes its work tree entry and drops the
                        stored module data. A pointer that moved inside the submodule is discarded.
                        This cannot be undone from the UI.`}
                    </DialogDescription>
                </DialogHeader>
                <DialogFooter>
                    <Button disabled={busy} onClick={onCancel} variant="outline">
                        Cancel
                    </Button>
                    <Button disabled={busy} onClick={onConfirm}>
                        Remove
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
