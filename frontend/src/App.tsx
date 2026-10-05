import {QueryClientProvider} from '@tanstack/react-query';
import {ResizableHandle, ResizablePanel, ResizablePanelGroup} from '@/components/ui/resizable';
import {Toaster} from '@/components/ui/sonner';
import {AppHeader} from '@/components/AppHeader';
import {AppSidebar} from '@/components/AppSidebar';
import {EmptyState} from '@/components/EmptyState';
import {MainTabs} from '@/components/MainTabs';
import {StatusBar} from '@/components/StatusBar';
import {SIDEBAR_PANEL_ID, useSidebarCollapse} from '@/hooks/useSidebarCollapse';
import {useSettings} from '@/hooks/useSettings';
import {useStatusEvents} from '@/hooks/useStatusEvents';
import {createQueryClient} from '@/lib/queryClient';
import {useRepoStore} from '@/stores/repo';
import {useState} from 'react';

// mounts the event bridge inside the query provider; renders nothing
function StatusEvents() {
    useStatusEvents();
    return null;
}

function App() {
    const [queryClient] = useState(createQueryClient);
    const repoOpen = useRepoStore((s) => s.isOpen);
    const sidebar = useSidebarCollapse();

    // loads persisted settings and applies the theme at startup (#12)
    useSettings();

    return (
        <QueryClientProvider client={queryClient}>
            <StatusEvents/>
            <div className="flex h-screen flex-col font-sans">
                <AppHeader sidebarOpen={sidebar.isOpen} onToggleSidebar={sidebar.toggle} />
                <ResizablePanelGroup direction="horizontal" className="min-h-0 flex-1">
                    <ResizablePanel
                        id={SIDEBAR_PANEL_ID}
                        defaultSize={22}
                        minSize={12}
                        maxSize={45}
                        collapsible
                        collapsedSize={0}
                        onCollapse={sidebar.onCollapse}
                        onExpand={sidebar.onExpand}
                        ref={sidebar.panelRef}
                    >
                        <AppSidebar />
                    </ResizablePanel>
                    <ResizableHandle withHandle />
                    <ResizablePanel minSize={40}>
                        <main className="flex h-full flex-col">{repoOpen ? <MainTabs /> : <EmptyState />}</main>
                    </ResizablePanel>
                </ResizablePanelGroup>
                <StatusBar />
            </div>
            <Toaster />
        </QueryClientProvider>
    );
}

export default App;
