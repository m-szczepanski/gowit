import {defineConfig} from 'vitest/config'
import react from '@vitejs/plugin-react'
import {fileURLToPath} from 'node:url'

export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: {
            '@': '/src'
        }
    },
    clearScreen: false,
    build: {
        // dist/.gitkeep is committed (main.go //go:embed needs the dir on a
        // fresh clone); the prebuild script removes stale output instead,
        // since emptyOutDir:true would wipe the tracked placeholder
        emptyOutDir: false
    },
    server: {
        // Wails discovers this port for hot-reload via "frontend:dev:serverUrl": "auto"
        port: 5173,
        strictPort: true
    },
    test: {
        globals: true,
        environment: 'jsdom',
        setupFiles: ['./vitest.setup.ts'],
        // vitest resolves react-resizable-panels' node build by default; its
        // no-op layout effect never registers panels, so collapse/expand
        // throws. Pin tests to the same browser build the app ships.
        alias: {
            'react-resizable-panels': fileURLToPath(new URL('./node_modules/react-resizable-panels/dist/react-resizable-panels.browser.esm.js', import.meta.url))
        },
        coverage: {
            include: ['src/**/*.ts', 'src/**/*.tsx'],
            // main.tsx is the entrypoint; components/ui is vendored shadcn code (registry-managed, not ours to test)
            exclude: ['src/main.tsx', 'src/vite-env.d.ts', 'src/components/ui/**'],
            thresholds: {
                lines: 100,
                functions: 100,
                branches: 100,
                statements: 100
            }
        }
    }
})
