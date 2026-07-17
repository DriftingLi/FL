<template>
  <div class="md-editor" :style="{ height: bodyHeight }">
    <!-- 模式切换 Tab -->
    <div class="md-mode-tabs">
      <button
        type="button"
        class="md-mode-tab"
        :class="{ active: mode === 'ir' }"
        @click="switchMode('ir')"
      >
        预览编辑
      </button>
      <button
        type="button"
        class="md-mode-tab"
        :class="{ active: mode === 'sv' }"
        @click="switchMode('sv')"
      >
        源码
      </button>
    </div>
    <!-- vditor 挂载点 -->
    <div ref="vditorRef" class="md-vditor-host"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
// vditor 是浏览器 DOM 库，动态导入避免 SSR/构建期问题
import Vditor from 'vditor'
import 'vditor/dist/index.css'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  height?: number
}>(), {
  placeholder: '请输入 Markdown 内容...',
  height: 500
})

const emit = defineEmits(['update:modelValue'])

const vditorRef = ref<HTMLElement | null>(null)
const mode = ref<'ir' | 'sv'>('ir')
let vditor: Vditor | null = null
// 防止 setValue 触发 input 回环
let internalUpdate = false

const bodyHeight = ref(`${props.height}px`)

// 创建 vditor 实例（切换模式时复用）
function createVditor(targetMode: 'ir' | 'sv') {
  if (!vditorRef.value) return
  // 销毁旧实例
  vditor?.destroy()
  vditor = null

  vditor = new Vditor(vditorRef.value, {
    height: props.height - 40, // 减去 tab 栏高度
    mode: targetMode,
    value: props.modelValue || '',
    placeholder: props.placeholder,
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
    preview: {
      hljs: {
        lineNumber: false,
        style: 'github'
      }
    },
    input: (value: string) => {
      if (internalUpdate) return
      emit('update:modelValue', value)
    }
  })
}

// 切换模式：ir=即时渲染（预览可编辑，obsidian 风格），sv=源码
function switchMode(target: 'ir' | 'sv') {
  if (mode.value === target) return
  // 切换前保存当前值
  const currentVal = vditor?.getValue() ?? props.modelValue
  internalUpdate = true
  mode.value = target
  createVditor(target)
  // 重建后恢复值
  nextTick(() => {
    if (vditor && currentVal) {
      vditor.setValue(currentVal, true)
    }
    internalUpdate = false
  })
}

onMounted(() => {
  createVditor('ir')
})

// 外部 modelValue 变化时同步到 vditor（避免回环）
watch(() => props.modelValue, (newVal) => {
  if (!vditor) return
  const current = vditor.getValue()
  if (newVal === current) return
  internalUpdate = true
  vditor.setValue(newVal || '')
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

.md-mode-tab:hover {
  color: #409eff;
}

.md-mode-tab.active {
  color: #409eff;
  border-bottom-color: #409eff;
  background: #fff;
}

.md-vditor-host {
  flex: 1;
  overflow: hidden;
}

/* 让 vditor 内部填满容器 */
.md-vditor-host :deep(.vditor) {
  border: none !important;
  border-radius: 0 !important;
}
</style>
