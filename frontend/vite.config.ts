import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    allowedHosts: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        timeout: 60000,
        proxyTimeout: 60000,
        ws: false
      },
      '/static': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        timeout: 60000,
        proxyTimeout: 60000
      }
    }
  }
})
