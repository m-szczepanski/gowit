import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {ResizableHandle, ResizablePanel, ResizablePanelGroup} from '@/components/ui/resizable';
import type {PanelId} from '@/stores/ui';
import {useUiStore} from '@/stores/ui';

export function MainTabs() {
    const activePanel = useUiStore((s) => s.activePanel);
    const setActivePanel = useUiStore((s) => s.setActivePanel);

    return (
        <Tabs
            className="flex h-full min-h-0"
            value={activePanel}
            onValueChange={(v) => setActivePanel(v as PanelId)}
        >
            <TabsList className="mx-3 mt-2">
                <TabsTrigger value="status">Status</TabsTrigger>
                <TabsTrigger value="history">History</TabsTrigger>
                <TabsTrigger value="graph">Graph</TabsTrigger>
            </TabsList>
            <TabsContent value="status" className="min-h-0 overflow-y-auto">
                <div data-testid="staging-view" className="p-3 text-sm text-muted-foreground">
                    No staged or unstaged changes
                </div>
            </TabsContent>
            <TabsContent value="history" className="flex min-h-0 flex-col">
                <ResizablePanelGroup direction="vertical" className="min-h-0 flex-1">
                    <ResizablePanel defaultSize={50} minSize={20}>
                        <div data-testid="commit-history" className="p-3 text-sm text-muted-foreground">
                            No commits loaded
                        </div>
                    </ResizablePanel>
                    <ResizableHandle withHandle />
                    <ResizablePanel defaultSize={50} minSize={20}>
                        <div data-testid="diff-viewer" className="p-3 font-mono text-sm text-muted-foreground">
                            Select a commit to view its diff
                        </div>
                    </ResizablePanel>
                </ResizablePanelGroup>
            </TabsContent>
            <TabsContent value="graph" className="min-h-0 overflow-y-auto">
                <div data-testid="commit-graph" className="p-3 text-sm text-muted-foreground">
                    No commits to graph
                </div>
            </TabsContent>
        </Tabs>
    );
}
