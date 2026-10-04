import './App.css';

function App() {
    return (
        <div className="app-shell">
            <aside id="sidebar" className="app-sidebar" data-testid="sidebar">
                Sidebar
            </aside>
            <main id="main-panel" className="app-main" data-testid="main-panel">
                Main panel
            </main>
        </div>
    );
}

export default App;
