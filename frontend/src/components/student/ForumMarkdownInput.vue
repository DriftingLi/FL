<script setup lang="ts">
/**
 * 论坛正文输入框（ADR-0052）：图一的 编写/预览 + Markdown 工具栏，图二的粘贴/拖拽区。
 *
 * 为什么是自建 textarea 而不是引 Vditor：ADR-0044 拒绝过一次（重依赖 + 预览走自有引擎，
 * 与发布结果不一致）。这里的预览走 `ForumContent` —— 与发布**同一个渲染单点**。
 *
 * 档位与可见性（ADR-0044「正文格式由作者声明」）：
 * - `format=text`（首访默认）：只有正文 + 粘贴区 + 字数，不出现 tab / 工具栏 / 提示行 ——
 *   纯文本没有可预览的渲染结果，摆一排禁用按钮只会制造误解；
 * - `format=markdown`：顶栏（左 编写|预览、右 工具栏）+ 正文 + 粘贴区 + 提示/计数。
 *
 * 撤销语义：工具栏插入优先走 `document.execCommand('insertText')`，为的是**保住浏览器
 * 原生撤销栈**（直接改 value 会把 Ctrl+Z 的语义弄坏）；不支持时回退到「写回 v-model +
 * 恢复选区」，此时原生撤销栈会丢 —— happy-dom 下走的就是回退路径，真实浏览器行为需人工确认。
 */
import { computed, nextTick, ref, watch } from 'vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiUnderlineTabs from '@/components/ui/UiUnderlineTabs.vue'
import MarkdownToolbar from '@/components/markdown/MarkdownToolbar.vue'
import ForumImageUploader from './ForumImageUploader.vue'
import ForumContent from './ForumContent.vue'
import { applyMarkdownCommand, type MarkdownCommandKey, type MarkdownEditResult } from '@/utils/markdownToolbar'
import { FORUM_MARKDOWN_HINT } from '@/utils/forumDisplay'

const props = withDefaults(
  defineProps<{
    modelValue: string
    /** 作者声明的正文格式（ADR-0044）：决定工具与预览是否出现 */
    format: 'text' | 'markdown'
    /** 已上传图片 URL（图文分离，正文里不内嵌图片） */
    images: string[]
    maxImages?: number
    maxlength?: number
    rows?: number
    placeholder?: string
  }>(),
  {
    maxImages: 3,
    maxlength: 5000,
    rows: 3,
    placeholder: '写下你的回复…'
  }
)

const emit = defineEmits<{
  'update:modelValue': [string]
  'update:images': [string[]]
  /** 透传 textarea 的 keydown：Ctrl/Cmd+Enter 提交由调用方决定 */
  keydown: [KeyboardEvent]
}>()

const isMarkdown = computed(() => props.format === 'markdown')

const content = computed({
  get: () => props.modelValue,
  set: (v: string) => emit('update:modelValue', v)
})

const images = computed({
  get: () => props.images,
  set: (v: string[]) => emit('update:images', v)
})

// ===== 编写 / 预览（视图档位，组件内自有状态）=====
type InputMode = 'write' | 'preview'
const MODE_OPTIONS: Array<{ label: string; value: string }> = [
  { label: '编写', value: 'write' },
  { label: '预览', value: 'preview' }
]
const mode = ref<InputMode>('write')
const previewing = computed(() => isMarkdown.value && mode.value === 'preview')

function onModeChange(value: string) {
  mode.value = value === 'preview' ? 'preview' : 'write'
}

// 格式切走（含切回纯文本）：复位到编写态，避免下次进 Markdown 就停在预览里
watch(
  () => props.format,
  () => {
    mode.value = 'write'
  }
)

// 回到编写态时把焦点交还正文：从预览切回来立刻能接着打字
watch(mode, async (next) => {
  if (next !== 'write') return
  await nextTick()
  inputRef.value?.focus?.()
})

// ===== 工具栏插入 =====
const inputRef = ref<{ focus?: () => void; getTextarea?: () => HTMLTextAreaElement | undefined } | null>(null)
const imageUploaderRef = ref<{ handlePaste?: (event: ClipboardEvent) => void } | null>(null)

/** 两串文本的最小差异区间：execCommand 只替换这一段，撤销栈里就是一步 */
function diffRange(before: string, after: string) {
  let from = 0
  const maxPrefix = Math.min(before.length, after.length)
  while (from < maxPrefix && before[from] === after[from]) from += 1
  let endBefore = before.length
  let endAfter = after.length
  while (endBefore > from && endAfter > from && before[endBefore - 1] === after[endAfter - 1]) {
    endBefore -= 1
    endAfter -= 1
  }
  return { from, endBefore, endAfter }
}

/**
 * 原生插入：把差异段选中再 `insertText`，浏览器会把它记成一次可撤销的编辑。
 * 返回 false 表示环境不支持或结果对不上，调用方走回退路径。
 */
function insertNatively(textarea: HTMLTextAreaElement, next: string, result: MarkdownEditResult): boolean {
  if (typeof document === 'undefined' || typeof document.execCommand !== 'function') return false
  try {
    const diff = diffRange(textarea.value, next)
    textarea.focus()
    textarea.setSelectionRange(diff.from, diff.endBefore)
    const ok = document.execCommand('insertText', false, next.slice(diff.from, diff.endAfter))
    // 有的环境 execCommand 返回 true 却什么也没做：以实际值为准
    if (ok === false || textarea.value !== next) return false
    textarea.setSelectionRange(result.start, result.end)
    return true
  } catch {
    return false
  }
}

function runCommand(command: MarkdownCommandKey) {
  if (previewing.value) return
  const textarea = inputRef.value?.getTextarea?.()
  // 事实源优先取原生元素的值：v-model 要经父级回环，极端情况下 props 会慢一拍
  const current = textarea ? textarea.value : (props.modelValue ?? '')
  // 拿不到原生元素（极早期）：退化成在文末追加，至少不吞掉这次点击
  const start = textarea ? textarea.selectionStart : current.length
  const end = textarea ? textarea.selectionEnd : current.length
  const result = applyMarkdownCommand(current, start, end, command)
  if (result.text === current) return

  if (textarea && insertNatively(textarea, result.text, result)) return

  emit('update:modelValue', result.text)
  void nextTick(() => {
    const el = inputRef.value?.getTextarea?.()
    el?.focus()
    el?.setSelectionRange(result.start, result.end)
  })
}

/** 粘贴转发给上传单点：只有焦点在本卡片内粘贴图片才被接管（不再挂 document） */
function onPaste(event: ClipboardEvent) {
  imageUploaderRef.value?.handlePaste?.(event)
}

defineExpose({
  focus: () => inputRef.value?.focus?.(),
  /** 提交/重置后复位到编写态：停在预览里只会看到一块空预览 */
  resetPreview: () => {
    mode.value = 'write'
  }
})
</script>

<template>
  <div
    class="forum-md-input rounded-[10px] border border-line bg-panel transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] focus-within:border-ui-500"
    @paste="onPaste"
  >
    <div
      v-if="isMarkdown"
      class="flex items-center justify-between gap-2 border-b border-line pr-1 pl-1.5"
    >
      <!--
        窄屏取舍：tab 不许被压（shrink-0），工具栏允许被压到容器宽后**横向滚动**
        （min-w-0 是让 flex 子项能缩到内容宽以下的关键，否则它会撑破卡片）。
      -->
      <UiUnderlineTabs class="shrink-0" :model-value="mode" :options="MODE_OPTIONS" @update:model-value="onModeChange" />
      <MarkdownToolbar :disabled="previewing" @command="runCommand" />
    </div>

    <div class="p-3">
      <!-- 预览与发布同源：都走 ForumContent 这一个 UGC 渲染单点 -->
      <ForumContent
        v-if="previewing"
        :content="props.modelValue"
        format="markdown"
        class="min-h-[60px] text-sm leading-[1.7] text-ink"
      />
      <UiInput
        v-else
        ref="inputRef"
        v-model="content"
        type="textarea"
        variant="bare"
        :rows="props.rows"
        :maxlength="props.maxlength"
        :placeholder="props.placeholder"
        @keydown="emit('keydown', $event)"
      />

      <ForumImageUploader ref="imageUploaderRef" v-model="images" :max="props.maxImages" class="mt-3" />

      <div class="mt-2 flex items-start gap-3">
        <p v-if="isMarkdown" class="forum-markdown-hint m-0 text-xs text-ink-3">
          {{ FORUM_MARKDOWN_HINT }}
        </p>
        <span class="forum-md-input-count ml-auto shrink-0 text-xs text-ink-3">
          {{ content.length }}/{{ props.maxlength }}
        </span>
      </div>
    </div>
  </div>
</template>
