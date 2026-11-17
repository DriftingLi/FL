<template>
  <div class="md-editor flex flex-col overflow-hidden rounded-ctl border border-line-strong bg-panel" :style="{ height: bodyHeight }">
    <!--
      模式切换 Tab：形态单点收在 UiUnderlineTabs（与论坛输入框同一件）——
      原来是本组件手搓的下划线 tab，属于全站第三处 tab 形态，#1017 一并收敛。
    -->
    <div class="md-mode-tabs flex shrink-0 border-b border-line bg-canvas px-1.5">
      <UiUnderlineTabs
        panel-id="md-editor-panel"
        :model-value="mode"
        :options="MODE_OPTIONS"
        :disabled="!isReady"
        @update:model-value="onModeChange"
      />
    </div>
    <!-- vditor 挂载点（也是模式的 tabpanel：tab 的 aria-controls 指向它） -->
    <div
      id="md-editor-panel"
      role="tabpanel"
      :aria-labelledby="`md-editor-panel-tab-${mode}`"
      ref="vditorRef"
      class="md-vditor-host flex-1 overflow-hidden relative"
    ></div>
    <!-- 加载中遮罩：Vditor 内部模块未就绪前覆盖，避免用户在未 ready 时切换模式触发 VditorIRDOM2Md undefined -->
    <div v-if="!isReady" class="md-loading absolute inset-0 z-10 flex items-center justify-center text-sm text-ink-3 pointer-events-all">
      <span>编辑器加载中…</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import UiUnderlineTabs from '@/components/ui/UiUnderlineTabs.vue'
import { useThemeStore } from '@/stores/theme'
// vditor 是浏览器 DOM 库，动态导入避免 SSR/构建期问题
import Vditor from 'vditor'
import 'vditor/dist/index.css'
// 深色适配：Vditor 自带一套浅色写死的配色（编辑区纯白），这里按 [data-theme] 换成 token
import '@/assets/styles/vditor-dark.css'
// 预加载中文语言包，挂载到 window.VditorI18n，避免 Vditor 运行时从 CDN (unpkg.com) 请求 i18n 文件导致 404
import 'vditor/dist/js/i18n/zh_CN.js'
import { useAuthStore } from '@/stores/auth'
// 内容排版按钮（颜色 / 字号 / 对齐 / 图片居中）：命令体是纯函数，按钮定义在独立文件里
import { createContentToolbarItems, createTableToolbarItems, type ContentToolbarItem } from './markdownEditorToolbar'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  height?: number
  /** 图片上传地址（绝对或相对路径，如 /api/admin/featured-content/upload-image）。
   *  传入后即可在编辑器内直接上传图片到后端，否则图片以 base64 嵌入。 */
  uploadUrl?: string
  /**
   * 是否显示内容排版按钮（对齐 / 文字颜色 / 字号 / 图片居中）。
   *
   * 默认关：这几个命令产出的是**内嵌 HTML**，而全站只有 Web 端会渲染它
   * （移动端解析器不认 HTML、论坛与 AI 面一律转义，见 ADR-0046）。
   * 只有「Web 章节正文」与「内容精选」这两个读者面才给开。
   */
  contentStyles?: boolean
}>(), {
  placeholder: '请输入 Markdown 内容...',
  height: 500,
  contentStyles: false
})

const emit = defineEmits(['update:modelValue'])

const vditorRef = ref<HTMLElement | null>(null)
const mode = ref<'ir' | 'sv'>('ir')

// 代码块高亮主题是 Vditor 自己按名字去拉样式表（github / github-dark），不跟着 CSS 变量走，
// 故这里按当前主题选一份；主题切换时重建实例（见文件尾的 watch）。
const themeStore = useThemeStore()
const isDark = computed(() => themeStore.resolved === 'dark')

/** Tab 选项（值与 Vditor 的 mode 同名：ir=即时渲染 / sv=源码） */
const MODE_OPTIONS = [
  { label: '预览编辑', value: 'ir' },
  { label: '源码', value: 'sv' }
]

/**
 * 内置工具栏（Vditor 预置项）。
 *
 * 内容排版那五个按钮（对齐 / 颜色 / 字号 / 图片居中）不是 Vditor 预置项，
 * 它们是 `IMenuItem` 形状的自定义项，由 `createContentToolbarItems` 生成后追加在
 * 表格与代码块之后 —— 位置选在那里，是因为它们和「表格 / 代码」同属「往正文里放结构」。
 */
const BASE_TOOLBAR = [
  'headings', 'bold', 'italic', 'strike', '|',
  'list', 'ordered-list', 'quote', 'line', '|',
  'link', 'table', 'code'
]

/**
 * 组装完整工具栏：内容排版按钮插在「表格 / 代码」之后、「撤销」之前
 *
 * @param targetMode 正在创建的模式：表格增删只在 IR 挂出 —— 源码模式没有表格 DOM，
 *   命令无处落脚（点了只会弹「请先把光标放进表格」的提示，不如不显示）。
 */
function buildToolbar(targetMode: 'ir' | 'sv'): Array<string | ContentToolbarItem> {
  const tail = ['|', 'undo', 'redo', 'fullscreen', 'preview']
  // getter 而不是实例：切模式 / 切主题都会重建实例，命令要拿到当下那一个
  // 表格增删行列**不受 contentStyles 门控**：它改的是 Markdown 表格结构、不产出内嵌 HTML，
  // 与那道「只有 Web 章节面才渲染 HTML」的闸门无关。
  const tableItems = targetMode === 'ir' ? createTableToolbarItems(() => vditor) : []
  if (!props.contentStyles) return [...BASE_TOOLBAR, ...tableItems, ...tail]
  return [...BASE_TOOLBAR, ...tableItems, ...createContentToolbarItems(() => vditor), ...tail]
}

/** UiUnderlineTabs 的载荷是 string，这里收窄回 Vditor 的两档 */
function onModeChange(value: string) {
  if (value === 'ir' || value === 'sv') switchMode(value)
}
// Vditor 是否真正初始化完成（after 回调触发后才算 ready）
// 在 ready 之前调用 getValue/setValue 会触发 VditorIRDOM2Md undefined 报错
const isReady = ref(false)
let vditor: Vditor | null = null
// 防止 setValue 触发 input 回环
let internalUpdate = false

const bodyHeight = ref(`${props.height}px`)

// 上传请求头（唯一形态：有令牌才带 Authorization）
function authHeaders(token: string): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {}
}

// 构造 Vditor upload 配置（仅当传入 uploadUrl 时启用）
//
// 令牌**按请求现取**（票 #1359 / spec #1345 真实缺陷 #11）：Vditor 走自己的 XHR，不经
// `api/client.ts` 的拦截器 ⇒ 401 不会触发静默刷新。术前的写法是在「创建编辑器」那一刻抓一份
// `authStore.token` 快照塞进 headers，而本组件常驻在章节编辑 / 精选正文编辑页里 —— 令牌一旦在
// 别处被刷新（store 不再缓存后 storage 才是事实源），这份快照就是过期令牌，插图直接 401。
// 现在两个钩子各管一半，且都不持有副本：
//   upload.file —— Vditor 发 xhr 前会 await 它（vditor/src/ts/upload/index.ts:187），
//                  在这里换一次新鲜 access token（本地过期则静默刷新，新令牌照 ADR-0016/0067
//                  的现有形态落进 storage，请求契约不改）；
//   upload.setHeaders —— 每次上传前同步调用（同文件 :230 → upload/setHeaders.ts:3），
//                  现取派生值 authStore.token（= 刚被上一步刷新过的那份 storage）。
function buildUploadConfig() {
  if (!props.uploadUrl) return undefined
  const authStore = useAuthStore()
  return {
    url: props.uploadUrl,
    fieldName: 'file',
    // 后端返回 { msg, code, data: { errFiles, succMap: { name: url } } }
    // Vditor 默认按此结构解析，无需额外处理
    accept: 'image/*',
    multiple: false,
    file: async (files: File[]) => {
      try {
        // 刻意不用返回值：副作用就是「把 storage 里的令牌换新鲜」，取头一律走 setHeaders
        await authStore.freshAccessToken()
      } catch {
        // 换取失败不拦上传：setHeaders 仍会带上当下 storage 里的那份
      }
      return files
    },
    setHeaders: () => authHeaders(authStore.token)
  }
}

/**
 * 全屏层级（修「全屏后退不出」）。
 *
 * Vditor 默认 `fullscreen.index = 90`，而应用顶栏 / 侧栏是 `var(--z-fixed)` = 1030
 * （`assets/styles/design-tokens.css`）—— 全屏后顶栏照旧压在编辑器之上，
 * 工具栏连同「退出全屏」那个按钮都被盖住、点不到。实测（临时探针 fs4.html）：
 * index=90 时 `document.elementFromPoint(按钮中心)` 命中顶栏（`btnReachable:false`），
 * index 抬高后命中按钮自身（`true`）。
 *
 * 取值 = `--z-fixed` + 1：高于顶栏、仍低于 `--z-modal-backdrop`(1040)，
 * 于是全屏期间弹出的确认框还盖得住编辑器（否则用户看不见它、以为卡死）。
 * 令牌在浏览器里读、读不到才退兜底 —— 写死数字会在令牌调整后重新被压住。
 */
function fullscreenIndex(): number {
  if (typeof document === 'undefined') return 1031
  const fixed = Number.parseInt(
    getComputedStyle(document.documentElement).getPropertyValue('--z-fixed'),
    10
  )
  return (Number.isFinite(fixed) ? fixed : 1030) + 1
}

/**
 * 释放全屏留下的滚动锁。
 *
 * Vditor 进全屏时把 `document.body.style.overflow` 设成 `hidden`，但它的
 * `destroy`（`vditor/dist/index.js`）只清自己元素的 innerHTML / class / style，
 * **不动 body** —— 在全屏态切模式或切主题（本组件 destroy + 重建）就会把整页
 * 永久锁死滚动。所以销毁全屏态实例前，由我们把这把锁还回去。
 */
function releaseBodyScrollLock(instance: Vditor | null) {
  if (!instance || typeof document === 'undefined') return
  if (instance.vditor?.element?.classList.contains('vditor--fullscreen')) {
    document.body.style.overflow = ''
  }
}

// 创建 vditor 实例（切换模式时复用）
// valueToRestore：切换模式时，等 after 回调触发后再用 setValue 恢复内容
function createVditor(targetMode: 'ir' | 'sv', valueToRestore?: string) {
  if (!vditorRef.value) return
  // 销毁旧实例
  releaseBodyScrollLock(vditor)
  vditor?.destroy()
  vditor = null
  // 重建期间禁止交互
  isReady.value = false

  vditor = new Vditor(vditorRef.value, {
    height: props.height - 40, // 减去 tab 栏高度
    mode: targetMode,
    value: props.modelValue || '',
    placeholder: props.placeholder,
    // 传入预加载的中文语言包，避免 Vditor 从 CDN (unpkg.com) 加载 i18n 文件导致 404
    i18n: (typeof window !== 'undefined' ? (window as any).VditorI18n : undefined) || undefined,
    // 使用本地 CDN 路径，避免从 jsdelivr/unpkg 远程加载 method.min.js 等运行时模块导致国内访问慢
    // 由 vite.config.ts 中的 vditor-static 插件把 node_modules/vditor/dist 复制到 /vditor/
    cdn: '/vditor',
    toolbar: buildToolbar(targetMode),
    // 全屏时编辑器要盖过应用顶栏（见 fullscreenIndex 的说明）
    fullscreen: { index: fullscreenIndex() },
    toolbarConfig: {
      pin: true
    },
    cache: { enable: false },
    // 仅当传入 uploadUrl 时覆盖默认 upload 配置，避免 undefined 覆盖导致 options.upload.url 报错
    ...(props.uploadUrl ? { upload: buildUploadConfig() } : {}),
    preview: {
      hljs: {
        lineNumber: false,
        style: isDark.value ? 'github-dark' : 'github'
      }
    },
    input: (value: string) => {
      if (internalUpdate) return
      emit('update:modelValue', value)
    },
    // Vditor 真正初始化完成（包括 IR/SV 模块加载）后触发
    // 在此之前调用 getValue 会报 VditorIRDOM2Md undefined
    after: () => {
      isReady.value = true
      if (valueToRestore && vditor) {
        try {
          vditor.setValue(valueToRestore, true)
        } catch (e) {
          console.warn('Vditor setValue 失败:', e)
        }
      }
      nextTick(() => { internalUpdate = false })
    }
  })
}

// 切换模式：ir=即时渲染（预览可编辑，obsidian 风格），sv=源码
function switchMode(target: 'ir' | 'sv') {
  if (mode.value === target) return
  // 未 ready 时禁止切换：Vditor 内部 IR/SV 模块未就绪，调用 getValue 会报 VditorIRDOM2Md undefined
  if (!vditor || !isReady.value) return

  // 切换前保存当前值（用 try/catch 兜底，避免极端情况下 getValue 抛错导致切换中断）
  let currentVal = props.modelValue
  try {
    currentVal = vditor.getValue() || props.modelValue
  } catch (e) {
    console.warn('Vditor getValue 失败，使用 props.modelValue:', e)
  }
  internalUpdate = true
  mode.value = target
  // 重建实例，等 after 回调触发后再恢复内容（避免在未 ready 时 setValue 报错）
  createVditor(target, currentVal)
}

onMounted(() => {
  createVditor('ir')
})

// 外部 modelValue 变化时同步到 vditor（避免回环）
watch(() => props.modelValue, (newVal) => {
  if (!vditor || !isReady.value) return
  // 用 try/catch 包裹 getValue，避免 ready 但 IR 模块短暂未就绪时抛错
  let current = ''
  try {
    current = vditor.getValue() || ''
  } catch {
    return
  }
  if (newVal === current) return
  internalUpdate = true
  try {
    vditor.setValue(newVal || '')
  } catch (e) {
    console.warn('Vditor setValue 失败:', e)
  }
  nextTick(() => { internalUpdate = false })
})

// 高度变化时更新容器
watch(() => props.height, (h) => {
  bodyHeight.value = `${h}px`
})

// 主题切换：Vditor 的高亮主题在初始化时就定死了，重建一次让它换成 github-dark / github。
// 内容从当前实例读出来再恢复（与模式切换同一手法），不改表单值、不触发 input 回环。
watch(isDark, () => {
  if (!vditor || !isReady.value) return
  let current = props.modelValue
  try {
    current = vditor.getValue() || props.modelValue
  } catch {
    // 读不出来就退回表单值：主题切换不值得为它中断编辑
  }
  internalUpdate = true
  createVditor(mode.value, current)
})

onBeforeUnmount(() => {
  releaseBodyScrollLock(vditor)
  vditor?.destroy()
  vditor = null
})

// 暴露给父组件：主动获取 Vditor 内部最新值
// 用于保存前兜底（避免 v-model 在某些输入场景下未及时同步）
defineExpose({
  getValue: (): string => {
    // 未 ready 时直接返回 props.modelValue，避免触发 VditorIRDOM2Md undefined
    if (!vditor || !isReady.value) return props.modelValue || ''
    try {
      return vditor.getValue() || ''
    } catch {
      return props.modelValue || ''
    }
  }
})
</script>

<style scoped>
/* R1 允许：EP/vditor 内部覆盖 + 半透明主题色底。其余样式已全部原子化。 */

.md-vditor-host :deep(.vditor) {
  border: none !important;
  border-radius: 0 !important;
}

/* 加载中遮罩底：color-mix 半透明卡色，深浅主题自动跟随（原 rgba(255,255,255,.85) 深色下会发白） */
.md-loading {
  background: color-mix(in srgb, var(--color-bg-card) 85%, transparent);
}
</style>
