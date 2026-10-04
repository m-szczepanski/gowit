import {defineConfig} from 'vite'
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
    }
})
