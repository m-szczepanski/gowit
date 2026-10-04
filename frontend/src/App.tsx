import {useState} from 'react';
import {QueryClientProvider} from '@tanstack/react-query';
import {Button} from '@/components/ui/button';
import {Toaster} from '@/components/ui/sonner';
import {ThemeDemo} from '@/components/ThemeDemo';
import {createQueryClient} from '@/lib/queryClient';
import {exampleBindOptions} from '@/lib/exampleBind';

function App() {
    const [queryClient] = useState(createQueryClient);
    const [bindResult, setBindResult] = useState('Go binding not called yet');

    const pingBackend = () => {
        queryClient
            .fetchQuery(exampleBindOptions)
            .then(setBindResult)
            .catch((err) => setBindResult(String(err)));
    };

    return (
        <QueryClientProvider client={queryClient}>
            <div className="flex h-screen font-sans">
                <aside className="w-72 shrink-0 overflow-y-auto border-r border-sidebar-border bg-sidebar p-4 text-sidebar-foreground" data-testid="sidebar">
                    Sidebar
                </aside>
                <main className="flex-1 overflow-y-auto bg-background" data-testid="main-panel">
                    <div className="flex items-center gap-2 border-b border-border p-4">
                        <p className="text-sm" data-testid="bind-result">
                            {bindResult}
                        </p>
                        <Button size="sm" onClick={pingBackend}>
                            Ping Go backend
                        </Button>
                    </div>
                    <ThemeDemo />
                </main>
                <Toaster />
            </div>
        </QueryClientProvider>
    );
}

export default App;
