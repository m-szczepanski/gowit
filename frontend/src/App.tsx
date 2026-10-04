import {useState} from 'react';
import {QueryClientProvider} from '@tanstack/react-query';
import {createQueryClient} from '@/lib/queryClient';

function App() {
    const [queryClient] = useState(createQueryClient);

    return (
        <QueryClientProvider client={queryClient}>
            <div className="flex h-screen font-sans">
                <aside className="w-72 shrink-0 overflow-y-auto border-r border-sidebar-border bg-sidebar p-4 text-sidebar-foreground" data-testid="sidebar">
                    Sidebar
                </aside>
                <main className="flex-1 overflow-y-auto bg-background p-4" data-testid="main-panel">
                    <p data-testid="shell-placeholder">gowit shell</p>
                </main>
            </div>
        </QueryClientProvider>
    );
}

export default App;
