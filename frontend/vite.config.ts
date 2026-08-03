import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'
import fs from 'node:fs'

/**
 * Vditor 本地 CDN 复制插件
 *
 * Vditor 的 cdn 选项指向 vditor 包根目录（默认 https://unpkg.com/vditor@3.11.2），
 * 内部会拼接 ${cdn}/dist/js/lute/lute.min.js 等路径加载运行时模块。
 * 本插件把 node_modules/vditor/dist 复制到 public/vditor/dist，
 * 配合 MarkdownEditor 中 `cdn: '/vditor'` 选项，让请求 /vditor/dist/js/...
 * 能命中 public/vditor/dist/js/... 实现本地加载，避免从 unpkg CDN 加载（国内慢且 404）。
 */
function vditorStaticPlugin(): Plugin {
  const src = path.resolve(__dirname, 'node_modules/vditor/dist')
  const dest = path.resolve(__dirname, 'public/vditor/dist')
  const copy = () => {
    if (!fs.existsSync(src)) return
    if (fs.existsSync(dest)) {
      // 已存在则跳过（避免每次启动都复制）
      return
    }
    fs.mkdirSync(path.resolve(__dirname, 'public/vditor'), { recursive: true })
    fs.cpSync(src, dest, { recursive: true })
  }
  return {
    name: 'vditor-static-copy',
    apply: () => true,
    configureServer() {
      copy()
    },
    buildStart() {
      copy()
    }
  }
}

export default defineConfig({
  plugins: [vue(), vditorStaticPlugin()],
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
  },
  build: {
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        // 按第三方库拆分 vendor chunk，避免 Element Plus / ECharts / PDF 等
        // 大依赖打进入口 chunk（此前两个入口 chunk 均超 1.1MB）
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('/element-plus/') || id.includes('@element-plus/')) return 'element-plus'
          if (id.includes('/echarts/') || id.includes('/zrender/')) return 'echarts'
          if (id.includes('/pdfjs-dist/')) return 'pdfjs'
          if (id.includes('/vditor/')) return 'vditor'
          if (id.includes('/marked') || id.includes('highlight.js')) return 'markdown'
          if (id.includes('/dayjs/')) return 'dayjs'
          if (id.includes('/vuedraggable/') || id.includes('/sortablejs/')) return 'draggable'
          if (id.includes('/vue') || id.includes('/pinia') || id.includes('/axios') || id.includes('/@vue/')) {
            return 'vue-vendor'
          }
          return 'vendor'
        }
      }
    }
  }
})
