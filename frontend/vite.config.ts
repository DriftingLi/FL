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
    fs.mkdirSync(dest, { recursive: true })
    // cpSync force 覆盖写，不做任何 rmSync —— 本机 safe-delete 的批量删除保护
    // 对 public/vditor/dist 树上的 rmSync 一律按整树计数（约 5000 文件 > 阈值 500），
    // 无论目标是目录还是单文件都会拦截启动（#768 期间实测两次 Startup Error）。
    // 白名单只增不减（见上注释），覆盖写无残留风险，语义等价「同步最新产物」。
    for (const item of WHITELIST) {
      const srcPath = path.join(src, item)
      if (!fs.existsSync(srcPath)) continue
      fs.cpSync(srcPath, path.join(dest, item), { recursive: true, force: true })
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
  plugins: [
    vue(),
    tailwindcss(),
    vditorStaticPlugin()],
  test: {
    environment: 'happy-dom',
    globals: true,
    // 并发下重挂载用例（CourseCatalog：整页 + el-table 全渲染）会被 CPU 争抢拖到
    // 5s 默认超时之外（单跑每例 <1.1s）。epLite 降低了 import 争抢但未根除，
    // 给裕量到 15s（#772：单跑健康 + 全量并发偶发超时的折中兜底）
    testTimeout: 15_000,
    // Node 25+ localStorage 遮蔽兜底（见 vitest.setup.ts）；Node ≤24 环境实现正常时零影响
    setupFiles: ['./vitest.setup.ts'],
    /*
     * 覆盖率（#768）：只做度量不做治理 —— 不设 thresholds、不挂 CI 门禁。
     * npx vitest run --coverage 产出 text + html 报告（coverage/ 目录）。
     * include 只算业务代码；测试文件与测试基建不进分母。
     * 用 istanbul 而非 v8 provider：v8 的 coverageFilesDirectory 在收尾时要
     * rmSync 约 5000 个中间文件，会触发本机 safe-delete 的批量删除保护。
     */
    coverage: {
      provider: 'istanbul',
      // all: 未被任何测试加载的文件也计 0% 入报告 —— 否则 baseline 会系统性偏乐观
      all: true,
      reporter: ['text', 'html'],
      reportsDirectory: 'coverage',
      // 本机 safe-delete 会拦截对 coverage/ 的一切 rmSync（回收站积压导致任何
      // 批量删除都触发 bulk guard）—— 关掉运行前清目录，报告文件覆盖写
      cleanOnRerun: false,
      include: ['src/**'],
      exclude: ['src/**/*.spec.ts', 'src/**/__tests__/**', 'src/test/**', 'src/main.ts']
    }
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
    // 关闭 modulePreload：vite 的 __vitePreload helper 会被 manualChunks 归入
    // markdown-stream 大 chunk，导致 entry 静态依赖整个 924KB（#748 性能评估 P0-1）。
    // 关闭后动态 import 裸加载，原生 module 图仍并行发现依赖；preload 提示的损失可接受。
    modulePreload: false,
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        // 按第三方库拆分 vendor chunk，避免 Element Plus / ECharts / PDF 等
        // 大依赖打进入口 chunk（此前两个入口 chunk 均超 1.1MB）
        manualChunks(id) {
          // vite 的 preload helper（虚拟模块，动态 import 注入 CSS 用）若不显式归置，
          // 会被自然聚进 markdown-stream 大 chunk，导致 entry 静态依赖整个 924KB（#748 P0-1）
          if (id.includes('vite/preload-helper')) return 'vendor'
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
