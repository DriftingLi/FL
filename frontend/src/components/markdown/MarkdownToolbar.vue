<script setup lang="ts">
/**
 * Markdown 工具栏（图一形态）：一排图标按钮，**悬停出中文提示**，点击发一条命令。
 *
 * 边界：
 * - 本组件只发 `command`：不认识 textarea、不碰正文——选区读写与撤销栈留在
 *   `ForumMarkdownInput`（那里才有原生元素）；
 * - 文案与图标的**唯一来源**是 `MARKDOWN_TOOLBAR_ITEMS`（utils/markdownToolbar）。
 *   这里不许另抄一份中文：抄一份迟早出现「提示写着加粗、按钮叫 Bold」这种漂移；
 * - 提示走 `UiTooltip`（仓里唯一的 tooltip 封装；深色气泡样式由 element-overrides.css
 *   的 `.el-popper.is-dark` 全局覆写），组件内零硬编码颜色。
 *
 * 置灰用 `aria-disabled` 而**不是**原生 `disabled`：原生 disabled 的按钮不派发鼠标事件，
 * 整排提示会在预览档下同时消失；`aria-disabled` 既保留可悬停可聚焦（读屏也读得到），
 * 又把点击挡在 `run()` 里。
 */
import {
  MARKDOWN_TOOLBAR_ITEMS,
  MARKDOWN_TOOLBAR_DIVIDERS,
  type MarkdownCommandKey
} from '@/utils/markdownToolbar'
import UiTooltip from '@/components/ui/UiTooltip.vue'
import MarkdownToolbarIcon from './MarkdownToolbarIcon.vue'

const props = withDefaults(
  defineProps<{
    /** 预览档等不可编辑态：按钮置灰且点击无效，但提示仍可悬停查看 */
    disabled?: boolean
  }>(),
  { disabled: false }
)

const emit = defineEmits<{ command: [MarkdownCommandKey] }>()

function run(key: MarkdownCommandKey) {
  if (props.disabled) return
  emit('command', key)
}
</script>

<template>
  <div
    class="markdown-toolbar flex shrink-0 items-center gap-0.5 overflow-x-auto [scrollbar-width:none]"
    role="toolbar"
    aria-label="Markdown 工具栏"
  >
    <template v-for="item in MARKDOWN_TOOLBAR_ITEMS" :key="item.key">
      <!-- 分组竖线（图一的分隔）：纯装饰，读屏忽略 -->
      <span
        v-if="MARKDOWN_TOOLBAR_DIVIDERS.includes(item.key)"
        class="markdown-toolbar-divider mx-0.5 h-4 w-px shrink-0 bg-line-strong"
        aria-hidden="true"
      />
      <UiTooltip :content="item.label" placement="bottom" :show-after="300">
        <button
          type="button"
          class="inline-flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[6px] border-0 bg-transparent text-ink-2 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)]"
          :class="disabled ? 'cursor-not-allowed opacity-50' : 'hover:bg-canvas hover:text-ink'"
          :aria-label="item.label"
          :aria-disabled="disabled ? 'true' : undefined"
          @click="run(item.key)"
        >
          <MarkdownToolbarIcon :name="item.icon" />
        </button>
      </UiTooltip>
    </template>
  </div>
</template>
