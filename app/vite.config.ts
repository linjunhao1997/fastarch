import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/authorization': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/account': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/application': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/authentication': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})