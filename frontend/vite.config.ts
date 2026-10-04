import {defineConfig} from 'vitest/config'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: {
            '@': '/src'
        }
    },
    clearScreen: false,
    server: {
        // Wails discovers this port for hot-reload via "frontend:dev:serverUrl": "auto"
        port: 5173,
        strictPort: true
    },
    test: {
        globals: true,
        environment: 'jsdom',
        coverage: {
            include: ['src/**'],
            exclude: ['src/main.tsx', 'src/vite-env.d.ts'],
            thresholds: {
                lines: 100,
                functions: 100,
                branches: 100,
                statements: 100
            }
        }
    }
})
