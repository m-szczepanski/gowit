import {Button} from '@/components/ui/button';
import {Checkbox} from '@/components/ui/checkbox';
import {ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuTrigger} from '@/components/ui/context-menu';
import {Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger} from '@/components/ui/dialog';
import {DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger} from '@/components/ui/dropdown-menu';
import {Input} from '@/components/ui/input';
import {ScrollArea} from '@/components/ui/scroll-area';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {Tooltip, TooltipContent, TooltipProvider, TooltipTrigger} from '@/components/ui/tooltip';

type PanelProps = {
    testId: string;
    title: string;
};

function PrimitivePanel({testId, title}: PanelProps) {
    return (
        <div data-testid={testId} className="flex flex-1 flex-col gap-4 rounded-lg border bg-card p-4 text-card-foreground">
            <h2 className="font-mono text-sm font-semibold uppercase tracking-wider">{title}</h2>

            <div className="flex flex-wrap gap-2">
                <Button>Primary</Button>
                <Button variant="secondary">Secondary</Button>
                <Button variant="outline">Outline</Button>
                <Button variant="destructive">Destructive</Button>
                <Button variant="ghost">Ghost</Button>
            </div>

            <div className="flex items-center gap-2">
                <Checkbox id={`${testId}-stage`} defaultChecked />
                <label htmlFor={`${testId}-stage`} className="text-sm">
                    Stage file
                </label>
                <Input placeholder="Commit message" className="w-48" />
            </div>

            <Tabs defaultValue="status" className="w-64">
                <TabsList>
                    <TabsTrigger value="status">Status</TabsTrigger>
                    <TabsTrigger value="log">Log</TabsTrigger>
                </TabsList>
                <TabsContent value="status">
                    <p className="text-sm">2 files changed</p>
                </TabsContent>
                <TabsContent value="log">
                    <p className="text-sm">a1b2c3 HEAD fix: thing</p>
                </TabsContent>
            </Tabs>

            <ScrollArea className="h-24">
                <div className="rounded-md border p-2">
                    {Array.from({length: 8}, (_, i) => (
                        <p key={i} className="font-mono text-xs leading-5">
                            line {i + 1}
                        </p>
                    ))}
                </div>
            </ScrollArea>

            <div className="font-mono text-xs">
                <p className="rounded bg-diff-add px-2 py-1 text-diff-add-foreground">+ added line</p>
                <p className="rounded bg-diff-remove px-2 py-1 text-diff-remove-foreground">- removed line</p>
            </div>

            <div className="flex flex-wrap items-center gap-2">
                <TooltipProvider>
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Button variant="outline" size="sm">
                                Tooltip
                            </Button>
                        </TooltipTrigger>
                        <TooltipContent>Push to origin/main</TooltipContent>
                    </Tooltip>
                </TooltipProvider>

                <Dialog>
                    <DialogTrigger asChild>
                        <Button variant="outline" size="sm">
                            Dialog
                        </Button>
                    </DialogTrigger>
                    <DialogContent>
                        <DialogHeader>
                            <DialogTitle>New branch</DialogTitle>
                        </DialogHeader>
                    </DialogContent>
                </Dialog>

                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="outline" size="sm">
                            Dropdown
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent>
                        <DropdownMenuItem>Fetch</DropdownMenuItem>
                        <DropdownMenuItem>Pull</DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>

                <ContextMenu>
                    <ContextMenuTrigger asChild>
                        <Button variant="outline" size="sm">
                            Right-click me
                        </Button>
                    </ContextMenuTrigger>
                    <ContextMenuContent>
                        <ContextMenuItem>Cherry-pick</ContextMenuItem>
                        <ContextMenuItem>Revert</ContextMenuItem>
                    </ContextMenuContent>
                </ContextMenu>
            </div>

            <div className="flex gap-2 text-xs font-medium">
                <span className="rounded bg-success px-2 py-1 text-success-foreground">success</span>
                <span className="rounded bg-warning px-2 py-1 text-warning-foreground">warning</span>
                <span className="rounded bg-destructive px-2 py-1 text-destructive-foreground">danger</span>
            </div>
        </div>
    );
}

export function ThemeDemo() {
    return (
        <div className="flex gap-4 p-4">
            <div className="light flex flex-1 flex-col">
                <PrimitivePanel testId="theme-demo-light" title="Light tokens" />
            </div>
            <div className="dark flex flex-1 flex-col">
                <PrimitivePanel testId="theme-demo-dark" title="Dark tokens" />
            </div>
        </div>
    );
}
