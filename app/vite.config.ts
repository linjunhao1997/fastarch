import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // 尾部斜杠避免匹配前端路由前缀（如 /authorization_management/...），
      // 否则刷新管理页面会被代理到后端返回 404
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
    },
  },
})