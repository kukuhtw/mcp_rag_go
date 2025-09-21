// vite.config.ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: { host: '0.0.0.0', port: 5173, proxy: {
    '/chat': 'http://localhost:8080',
    '/timeseries': 'http://localhost:8080',
    '/healthz': 'http://localhost:8080'
  }}
})
