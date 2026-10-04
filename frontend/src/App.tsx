import {useState} from 'react';
import './App.css';
import {ExampleBind} from '../wailsjs/go/main/App';

function App() {
    const [bindResult, setBindResult] = useState('Go binding not called yet');

    const pingBackend = () => {
        ExampleBind()
            .then(setBindResult)
            .catch((err) => setBindResult(String(err)));
    };

    return (
        <div className="app-shell">
            <aside id="sidebar" className="app-sidebar" data-testid="sidebar">
                Sidebar
            </aside>
            <main id="main-panel" className="app-main" data-testid="main-panel">
                <p data-testid="bind-result">{bindResult}</p>
                <button onClick={pingBackend}>Ping Go backend</button>
            </main>
        </div>
    );
}

export default App;
