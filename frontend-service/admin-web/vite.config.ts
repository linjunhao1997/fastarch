import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': new URL('./src', import.meta.url).pathname,
    },
  },
  server: {
    // Windows 会把 5148-5247 等区段列为「保留端口」(Hyper-V/WSL)，
    // 5173 落在其中会 listen EACCES；固定用保留区之外的 4173。
    host: '127.0.0.1',
    port: 4173,
    strictPort: true,
    proxy: {
      '/authorization/': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/account/': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/application/': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/authentication/': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      '/iam/': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
      // sender 服务网关；路由 path /sender/records 与 API 前缀重合，
      // HTML 导航请求（浏览器直访/刷新）必须放行给前端 SPA，只代理 API。
      '/sender/': {
        target: 'http://127.0.0.1:8081',
        changeOrigin: true,
        bypass(req) {
          const accept = req.headers?.accept ?? '';
          const path = (req as { url?: string }).url ?? '';
          // 接受 text/html 且不是明确的 API 路径 → 交给前端路由
          if (typeof accept === 'string' && accept.includes('text/html') && !/\.(json|pb)$/.test(path)) {
            return '/index.html';
          }
          return undefined;
        },
      },
      // flow 服务网关（任务工作台）；/flow/tasks 既是页面路由又是 API 前缀，
      // HTML 导航请求必须放行给前端 SPA，只代理 API 调用。
      '/flow/': {
        target: 'http://127.0.0.1:8082',
        changeOrigin: true,
        bypass(req) {
          const accept = req.headers?.accept ?? '';
          const path = (req as { url?: string }).url ?? '';
          if (typeof accept === 'string' && accept.includes('text/html') && !/\.(json|pb)$/.test(path)) {
            return '/index.html';
          }
          return undefined;
        },
      },
      // home 服务网关（企业官网）；/home/homepage 既是页面路由又是 API 前缀，
      // HTML 导航请求必须放行给前端 SPA，只代理 API 调用。
      '/home/': {
        target: 'http://127.0.0.1:8083',
        changeOrigin: true,
        bypass(req) {
          const accept = req.headers?.accept ?? '';
          const path = (req as { url?: string }).url ?? '';
          if (typeof accept === 'string' && accept.includes('text/html') && !/\.(json|pb)$/.test(path)) {
            return '/index.html';
          }
          return undefined;
        },
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
          'antd': ['antd', '@ant-design/icons'],
          'tanstack': ['@tanstack/react-query'],
        },
      },
    },
  },
})