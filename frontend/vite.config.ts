import { defineConfig, type Plugin } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'
import fs from 'node:fs'

/**
 * Vditor 本地 CDN 复制插件（白名单精简版）
 *
 * Vditor 的 cdn 选项指向 vditor 包根目录（默认 https://unpkg.com/vditor@3.11.2），
 * 内部会拼接 ${cdn}/dist/js/lute/lute.min.js 等路径加载运行时模块。
 * 本插件把 node_modules/vditor/dist 中运行时实际需要的文件复制到 public/vditor/dist，
 * 配合 MarkdownEditor 中 `cdn: '/vditor'` 选项，让请求 /vditor/dist/js/...
 * 能命中 public/vditor/dist/js/... 实现本地加载，避免从 unpkg CDN 加载（国内慢且 404）。
 *
 * 只复制 WHITELIST 里的路径（全量复制约 23MB，白名单后约 6MB）：
 * - 必选：index.css（预览 iframe 样式）、method.min.js（预览渲染模块）、js/lute（Markdown 解析内核）、js/i18n（语言包）
 * - 默认开启：js/katex（preview.math 默认 KaTeX）、js/highlight.js 的 highlight.min.js + third-languages.js + github 样式（preview.hljs）、js/icons/ant.js（默认 icon: 'ant'）
 * - 未启用则不复制：emoji、content-theme、以及按内容懒加载的可选渲染引擎
 *   （mermaid / mathjax / echarts / graphviz / markmap / abcjs / flowchart.js / plantuml / smiles-drawer）。
 *   若未来内容需要这些引擎，往 WHITELIST 加对应目录即可。
 */
function vditorStaticPlugin(): Plugin {
  const src = path.resolve(__dirname, 'node_modules/vditor/dist')
  const dest = path.resolve(__dirname, 'public/vditor/dist')
  const WHITELIST = [
    'index.css',
    'method.min.js',
    'js/i18n',
    'js/icons',
    'js/lute/lute.min.js',
    'js/katex',
    'js/highlight.js/highlight.min.js',
    'js/highlight.js/third-languages.js',
    'js/highlight.js/styles/github.min.css',
    'images/logo.png'
  ]
  const copy = () => {
    if (!fs.existsSync(src)) return
    // 每次全量同步：先清空旧产物，再按白名单复制，避免残留大体积文件
    fs.rmSync(dest, { recursive: true, force: true })
    fs.mkdirSync(dest, { recursive: true })
    for (const item of WHITELIST) {
      const srcPath = path.join(src, item)
      if (!fs.existsSync(srcPath)) continue
      fs.cpSync(srcPath, path.join(dest, item), { recursive: true })
    }
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
  plugins: [vue(), tailwindcss(), vditorStaticPlugin()],
  test: {
    environment: 'happy-dom',
    globals: true,
    // Node 25+ localStorage 遮蔽兜底（见 vitest.setup.ts）；Node ≤24 环境实现正常时零影响
    setupFiles: ['./vitest.setup.ts']
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    allowedHosts: true,
    // WSL /mnt/d（drvfs）下 inotify 不可靠，轮询监听保证 HMR 生效
    watch: {
      usePolling: true,
      interval: 200
    },
    proxy: {
      // 后端容器宿主端口 18080（8080 落在 Windows Hyper-V 排除端口段 8025-8124 内，见 backend/docker-compose.yml 注释）
      '/api': {
        target: 'http://127.0.0.1:18080',
        changeOrigin: true,
        timeout: 60000,
        proxyTimeout: 60000,
        ws: false
      },
      '/static': {
        target: 'http://127.0.0.1:18080',
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
          if (id.includes('markstream-vue') || id.includes('markstream-core') || id.includes('stream-markdown-parser')) return 'markdown-stream'
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
