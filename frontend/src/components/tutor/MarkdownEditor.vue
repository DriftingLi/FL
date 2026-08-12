<template>
  <div class="md-editor" :style="{ height: bodyHeight }">
    <!-- 模式切换 Tab -->
    <div class="md-mode-tabs">
      <button
        type="button"
        class="md-mode-tab"
        :class="{ active: mode === 'ir' }"
        :disabled="!isReady"
        @click="switchMode('ir')"
      >
        预览编辑
      </button>
      <button
        type="button"
        class="md-mode-tab"
        :class="{ active: mode === 'sv' }"
        :disabled="!isReady"
        @click="switchMode('sv')"
      >
        源码
      </button>
    </div>
    <!-- vditor 挂载点 -->
    <div ref="vditorRef" class="md-vditor-host"></div>
    <!-- 加载中遮罩：Vditor 内部模块未就绪前覆盖，避免用户在未 ready 时切换模式触发 VditorIRDOM2Md undefined -->
    <div v-if="!isReady" class="md-loading">
      <span>编辑器加载中…</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
// vditor 是浏览器 DOM 库，动态导入避免 SSR/构建期问题
import Vditor from 'vditor'
import 'vditor/dist/index.css'
// 预加载中文语言包，挂载到 window.VditorI18n，避免 Vditor 运行时从 CDN (unpkg.com) 请求 i18n 文件导致 404
import 'vditor/dist/js/i18n/zh_CN.js'
import { useAuthStore } from '@/stores/auth'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  height?: number
  /** 图片上传地址（绝对或相对路径，如 /api/admin/featured-content/upload-image）。
   *  传入后即可在编辑器内直接上传图片到后端，否则图片以 base64 嵌入。 */
  uploadUrl?: string
}>(), {
  placeholder: '请输入 Markdown 内容...',
  height: 500
})

const emit = defineEmits(['update:modelValue'])

const vditorRef = ref<HTMLElement | null>(null)
const mode = ref<'ir' | 'sv'>('ir')
// Vditor 是否真正初始化完成（after 回调触发后才算 ready）
// 在 ready 之前调用 getValue/setValue 会触发 VditorIRDOM2Md undefined 报错
const isReady = ref(false)
let vditor: Vditor | null = null
// 防止 setValue 触发 input 回环
let internalUpdate = false

const bodyHeight = ref(`${props.height}px`)

// 构造 Vditor upload 配置（仅当传入 uploadUrl 时启用）
function buildUploadConfig() {
  if (!props.uploadUrl) return undefined
  const authStore = useAuthStore()
  const headers: Record<string, string> = {}
  if (authStore.token) {
    headers.Authorization = `Bearer ${authStore.token}`
  }
  return {
    url: props.uploadUrl,
    fieldName: 'file',
    headers,
    // 后端返回 { msg, code, data: { errFiles, succMap: { name: url } } }
    // Vditor 默认按此结构解析，无需额外处理
    accept: 'image/*',
    multiple: false
  }
}

// 创建 vditor 实例（切换模式时复用）
// valueToRestore：切换模式时，等 after 回调触发后再用 setValue 恢复内容
function createVditor(targetMode: 'ir' | 'sv', valueToRestore?: string) {
  if (!vditorRef.value) return
  // 销毁旧实例
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
    toolbar: [
      'headings', 'bold', 'italic', 'strike', '|',
      'list', 'ordered-list', 'quote', 'line', '|',
      'link', 'table', 'code', '|',
      'undo', 'redo', 'fullscreen', 'preview'
    ],
    toolbarConfig: {
      pin: true
    },
    cache: { enable: false },
    // 仅当传入 uploadUrl 时覆盖默认 upload 配置，避免 undefined 覆盖导致 options.upload.url 报错
    ...(props.uploadUrl ? { upload: buildUploadConfig() } : {}),
    preview: {
      hljs: {
        lineNumber: false,
        style: 'github'
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

onBeforeUnmount(() => {
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
.md-editor {
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  background: #fff;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.md-mode-tabs {
  display: flex;
  border-bottom: 1px solid #ebeef5;
  background: #fafbfc;
  flex-shrink: 0;
}

.md-mode-tab {
  padding: 8px 18px;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: 14px;
  color: #606266;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
  font-family: inherit;
}

.md-mode-tab:hover:not(:disabled) {
  color: #409eff;
}

.md-mode-tab:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.md-mode-tab.active {
  color: #409eff;
  border-bottom-color: #409eff;
  background: #fff;
}

.md-vditor-host {
  flex: 1;
  overflow: hidden;
  position: relative;
}

/* 让 vditor 内部填满容器 */
.md-vditor-host :deep(.vditor) {
  border: none !important;
  border-radius: 0 !important;
}

/* 加载中遮罩：覆盖编辑器区域，防止 Vditor 未 ready 时用户误操作 */
.md-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.85);
  color: #909399;
  font-size: 14px;
  z-index: 10;
  pointer-events: all;
}
</style>
