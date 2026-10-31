<script setup lang="ts">
/**
 * 高度折叠容器（grid 0fr → 1fr）：靠外层轨道高度与透明度过渡，
 * 不依赖 JS 测量，也不像 v-show 那样瞬时跳变。
 *
 * 关闭时内层带 `inert`：高度压到 0 之后，里面的链接 / 按钮不能再被 Tab 进去。
 * `lazy` 用于内容较重的面板：首次展开前不挂载 slot，避免「默认折叠的消息来源」
 * 在每个历史气泡里都先跑一遍 Markdown 渲染；首次展开后保留 DOM，收起时仍有离场高度。
 * 这是「折叠态」的通用语义件；el-collapse 仍用于需要手风琴交互的面板，二者不互相替代。
 */
import { ref, watch } from 'vue'

const props = defineProps<{
  /** 展开态；v-model 不适用（状态由调用方持有，这里只渲染）。 */
  open: boolean
  /** 首次展开前不挂载内容（默认 false：侧栏分组这类轻量结构常驻）。 */
  lazy?: boolean
}>()

const mounted = ref(props.lazy !== true || props.open)
watch(
  () => props.open,
  (open) => {
    if (open) mounted.value = true
  }
)
</script>

<template>
  <div
    class="grid transition-[grid-template-rows] duration-[var(--duration-normal)] ease-[var(--ease-default)]"
    :class="open ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'"
  >
    <div
      class="min-h-0 overflow-hidden transition-opacity duration-[var(--duration-normal)] ease-[var(--ease-default)]"
      :class="open ? 'opacity-100' : 'opacity-0'"
      :inert="!open || undefined"
    >
      <slot v-if="mounted" />
    </div>
  </div>
</template>
