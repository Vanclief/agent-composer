import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// Where /api goes in `npm run dev` and `vite preview`: the mock by default,
// or a real agc (VITE_API_TARGET=http://127.0.0.1:1202 npm run dev).
const apiTarget = process.env.VITE_API_TARGET ?? 'http://127.0.0.1:8790'

// changeOrigin stays false: agc's terminal WebSocket only accepts an Origin
// that matches the Host header, and the browser's Origin is the Vite host.
const proxy = {
  '/api': { target: apiTarget, ws: true },
}

// emptyOutDir wipes dist/, including the committed dist/.keep that lets Go
// embed the directory before the panel is built. Put it back.
function keepDist(): Plugin {
  return {
    name: 'agc-keep-dist',
    apply: 'build',
    closeBundle() {
      writeFileSync(fileURLToPath(new URL('./dist/.keep', import.meta.url)), '')
    },
  }
}

export default defineConfig({
  base: '/panel/',
  plugins: [react(), tailwindcss(), keepDist()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: { proxy },
  preview: { proxy },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
