import {useRef, useState} from 'react';
import {useVirtualizer} from '@tanstack/react-virtual';
import {Trash2} from 'lucide-react';
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
import {useStatus} from '@/hooks/useStatus';
import {useDiscardFiles, useStageAll, useStageFiles, useUnstageAll, useUnstageFiles} from '@/hooks/useStaging';
import type {StatusResponse} from '@/lib/api';
import type {git} from '../../wailsjs/go/models';

type FileStatus = git.FileStatus;
type SectionKey = 'staged' | 'changes' | 'untracked' | 'conflicted';
type Section = {key: SectionKey; title: string; files: FileStatus[]};
type Item = {kind: 'section'; section: Section} | {kind: 'file'; section: SectionKey; file: FileStatus};

const ROW_HEIGHT = 36;

function label(change: string): string {
    return change.charAt(0).toUpperCase() + change.slice(1).replace('-', ' ');
}

function displayPath(f: FileStatus): string {
    return f.origPath ? `${f.origPath} -> ${f.path}` : f.path;
}

// Ignored entries never render as rows (Status omits them unless --ignored
// is passed, and the UI never requests that); conflicted files get their
// own read-only section per ARCHITECTURE §8 staging groups.
function group(files: FileStatus[]): Section[] {
    const sections: Section[] = [
        {key: 'staged', title: 'Staged', files: files.filter((f) => f.staged && !f.conflict)},
        {key: 'changes', title: 'Changes', files: files.filter((f) => f.unstaged && !f.conflict)},
        {key: 'untracked', title: 'Untracked', files: files.filter((f) => f.untracked)},
        // conflicts stay a read-only placeholder until the merge phase
        {key: 'conflicted', title: 'Conflicts', files: files.filter((f) => f.conflict)}
    ];
    return sections.filter((s) => s.files.length > 0);
}

function branchText(res: StatusResponse): string {
    const b = res.branch;
    const name = b.detached ? `(detached at ${b.oid.slice(0, 8)})` : b.head;
    const counts: string[] = [];
    if (b.ahead > 0) {
        counts.push(`↑${b.ahead}`);
    }
    if (b.behind > 0) {
        counts.push(`↓${b.behind}`);
    }
    const text = [name, ...counts].filter(Boolean).join(' ');
    return text === '' ? '(unborn)' : text;
}

export function StagingView() {
    const {data, error} = useStatus();
    const stageFiles = useStageFiles();
    const unstageFiles = useUnstageFiles();
    const stageAll = useStageAll();
    const unstageAll = useUnstageAll();
    const discard = useDiscardFiles();
    const scrollRef = useRef<HTMLDivElement>(null);
    const [discardPaths, setDiscardPaths] = useState<string[] | null>(null);

    const busy = stageFiles.isPending || unstageFiles.isPending || stageAll.isPending || unstageAll.isPending || discard.isPending;
    const items: Item[] = [];
    for (const section of group(data?.files ?? [])) {
        items.push({kind: 'section', section});
        for (const file of section.files) {
            items.push({kind: 'file', section: section.key, file});
        }
    }

    const virtualizer = useVirtualizer({
        count: items.length,
        getScrollElement: () => scrollRef.current,
        estimateSize: () => ROW_HEIGHT,
        overscan: 12,
        // before the first ResizeObserver pass there is no measured rect;
        // without a seed the list renders nothing on paint
        initialRect: {width: 1000, height: 1000}
    });

    if (error) {
        return (
            <div data-testid="staging-error" className="p-3 text-sm text-destructive">
                {`${(error as {code?: string}).code ?? 'error'}: ${error.message}`}
            </div>
        );
    }

    if (!data) {
        return <div data-testid="staging-loading" className="p-3 text-sm text-muted-foreground">Loading status…</div>;
    }

    // A file in a section is acted on from that section's perspective: the
    // staged row unstages, changes/untracked rows stage. MM files appear in
    // both sections and each row does the opposite job.
    const toggle = (f: FileStatus, fromSection: SectionKey) => {
        const paths = [f.path];
        if (fromSection === 'staged') {
            unstageFiles.mutate(paths);
        } else {
            stageFiles.mutate(paths);
        }
    };

    const toggleSection = (section: Section) => {
        const paths = section.files.map((f) => f.path);
        if (section.key === 'staged') {
            unstageFiles.mutate(paths);
        } else {
            stageFiles.mutate(paths);
        }
    };

    return (
        <div className="flex h-full min-h-0 flex-col" data-testid="staging-view">
            <div className="flex items-center justify-between gap-2 px-3 py-2">
                <span data-testid="staging-branch" className="truncate text-sm font-medium">
                    {branchText(data)}
                </span>
                <div className="flex shrink-0 gap-2">
                    <Button size="sm" variant="outline" disabled={busy} onClick={() => stageAll.mutate()}>
                        Stage All
                    </Button>
                    <Button size="sm" variant="outline" disabled={busy} onClick={() => unstageAll.mutate()}>
                        Unstage All
                    </Button>
                </div>
            </div>
            {items.length === 0 ? (
                <div data-testid="staging-empty" className="p-3 text-sm text-muted-foreground">
                    No local changes
                </div>
            ) : (
                <div data-virtual-scroll="" ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto">
                    <div
                        className="relative w-full"
                        style={{
                            // eslint-disable-next-line shadcn/no-inline-styles
                            height: virtualizer.getTotalSize()
                        }}
                    >
                        {virtualizer.getVirtualItems().map((virtualRow) => {
                            const item = items[virtualRow.index];
                            return (
                                <div
                                    className="absolute left-0 top-0 w-full"
                                    data-index={virtualRow.index}
                                    key={virtualRow.key}
                                    ref={virtualizer.measureElement}
                                    style={{
                                        // eslint-disable-next-line shadcn/no-inline-styles
                                        transform: `translateY(${virtualRow.start}px)`
                                    }}
                                >
                                    {item.kind === 'section' ? (
                                        <SectionHeader
                                            section={item.section}
                                            busy={busy}
                                            onToggleAll={() => toggleSection(item.section)}
                                        />
                                    ) : (
                                        <FileRow
                                            file={item.file}
                                            section={item.section}
                                            busy={busy}
                                            onToggle={() => toggle(item.file, item.section)}
                                            onDiscard={() => setDiscardPaths([item.file.path])}
                                        />
                                    )}
                                </div>
                            );
                        })}
                    </div>
                </div>
            )}
            <DiscardDialog
                busy={discard.isPending}
                onCancel={() => setDiscardPaths(null)}
                onConfirm={(list) => {
                    discard.mutate(list);
                    setDiscardPaths(null);
                }}
                open={discardPaths !== null}
                paths={discardPaths ?? []}
            />
        </div>
    );
}

function SectionHeader({section, busy, onToggleAll}: {section: Section; busy: boolean; onToggleAll: () => void}) {
    const interactive = section.key !== 'conflicted';
    return (
        <div
            className="flex h-9 items-center gap-2 bg-muted px-3"
            data-section={section.key}
            data-testid={`section-${section.key}`}
        >
            {interactive ? (
                <Checkbox
                    aria-label={`${section.key === 'staged' ? 'Unstage all ' : 'Stage all '}${section.title.toLowerCase()}`}
                    checked={section.key === 'staged'}
                    disabled={busy}
                    onCheckedChange={onToggleAll}
                />
            ) : (
                // conflicts need resolve/revert actions from the merge phase
                <span aria-hidden className="w-4" data-testid="section-conflicted-placeholder" />
            )}
            <span className="text-xs font-semibold uppercase">{section.title}</span>
            <span className="text-xs text-muted-foreground">{section.files.length}</span>
        </div>
    );
}

function FileRow({file, section, busy, onToggle, onDiscard}: {
    file: FileStatus;
    section: SectionKey;
    busy: boolean;
    onToggle: () => void;
    onDiscard: () => void;
}) {
    // The checkbox is the row's primary control: natively focusable, so
    // space/enter toggle it (button activation semantics) without any
    // hand-rolled row key handling stealing input from the discard button.
    return (
        <div
            className="flex h-9 items-center gap-2 px-3 hover:bg-accent"
            data-section={section}
            data-testid={`row-${section}-${file.path}`}
        >
            <Checkbox
                aria-label={`${section === 'staged' ? 'Unstage' : 'Stage'} ${file.path}`}
                checked={section === 'staged'}
                disabled={busy || file.conflict}
                onCheckedChange={onToggle}
            />
            <Badge variant="secondary">{file.xy}</Badge>
            <span className="w-24 shrink-0 text-xs text-muted-foreground">{label(file.change)}</span>
            <span className="truncate text-sm">{displayPath(file)}</span>
            {!file.conflict && (
                <Button
                    aria-label={`Discard ${file.path}`}
                    disabled={busy}
                    size="icon"
                    variant="ghost"
                    onClick={onDiscard}
                >
                    <Trash2 aria-hidden size={14} />
                </Button>
            )}
        </div>
    );
}

function DiscardDialog({open, paths, busy, onCancel, onConfirm}: {
    open: boolean;
    paths: string[];
    busy: boolean;
    onCancel: () => void;
    onConfirm: (paths: string[]) => void;
}) {
    // only close transitions can fire onOpenChange while the dialog is up
    return (
        <Dialog onOpenChange={onCancel} open={open}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>Discard changes?</DialogTitle>
                    <DialogDescription>
                        {`This reverts ${paths.length} file(s) and deletes untracked ones. Cannot be undone.`}
                    </DialogDescription>
                </DialogHeader>
                <ul className="max-h-40 overflow-y-auto text-sm">
                    {paths.map((p) => (
                        <li key={p} className="truncate">{p}</li>
                    ))}
                </ul>
                <DialogFooter>
                    <Button variant="outline" onClick={onCancel}>Cancel</Button>
                    <Button data-testid="discard-confirm" disabled={busy} variant="destructive" onClick={() => onConfirm(paths)}>
                        Discard
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
