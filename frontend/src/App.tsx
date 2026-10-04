import {useState} from 'react';
import {Button} from '@/components/ui/button';
import {Toaster} from '@/components/ui/sonner';
import {ThemeDemo} from '@/components/ThemeDemo';
import {ExampleBind} from '../wailsjs/go/main/App';

function App() {
    const [bindResult, setBindResult] = useState('Go binding not called yet');

    const pingBackend = () => {
        ExampleBind()
            .then(setBindResult)
            .catch((err) => setBindResult(String(err)));
    };

    return (
        <div className="grid h-screen grid-cols-[280px_1fr] font-sans">
            <aside className="overflow-y-auto border-r border-sidebar-border bg-sidebar p-4 text-sidebar-foreground" data-testid="sidebar">
                Sidebar
            </aside>
            <main className="overflow-y-auto bg-background" data-testid="main-panel">
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
    );
}

export default App;
