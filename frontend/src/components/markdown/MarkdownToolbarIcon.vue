<script setup lang="ts">
/**
 * Markdown 工具栏图标（内联 SVG / 字形，16 视角）。
 *
 * 为什么自己画：Element Plus 图标集里没有 加粗/斜体/引用/列表 这一套**文字排版**图标，
 * 拿通用图标凑会语义不清、粗细不一。字形类（H3 / B / I）直接用文字（同图一的做法），
 * 结构类用描边 SVG。
 *
 * 颜色一律 `currentColor`：深浅两套主题随文字色走，组件内零硬编码色值。
 */
defineProps<{
  /** 取值见 utils/markdownToolbar 的 MARKDOWN_TOOLBAR_ITEMS[].icon */
  name: string
}>()
</script>

<template>
  <span v-if="name === 'heading3'" class="text-[11px] leading-none font-semibold tracking-tight">H3</span>
  <span v-else-if="name === 'bold'" class="text-[13px] leading-none font-bold">B</span>
  <span v-else-if="name === 'italic'" class="font-serif text-[13px] leading-none italic">I</span>
  <svg
    v-else
    class="size-4 shrink-0"
    viewBox="0 0 16 16"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <template v-if="name === 'quote'">
      <path d="M3 3.5v9" />
      <path d="M6.5 5.5h6.5M6.5 10.5h4.5" />
    </template>
    <template v-else-if="name === 'code'">
      <path d="M5.5 4.5 2.5 8l3 3.5M10.5 4.5l3 3.5-3 3.5" />
    </template>
    <template v-else-if="name === 'link'">
      <path d="M6.6 9.4a2.7 2.7 0 0 1 0-3.8l1.9-1.9a2.7 2.7 0 0 1 3.8 3.8l-1 1" />
      <path d="M9.4 6.6a2.7 2.7 0 0 1 0 3.8l-1.9 1.9a2.7 2.7 0 0 1-3.8-3.8l1-1" />
    </template>
    <template v-else-if="name === 'ul'">
      <path d="M6 4h7M6 8h7M6 12h7" />
      <circle cx="3" cy="4" r="0.9" fill="currentColor" stroke="none" />
      <circle cx="3" cy="8" r="0.9" fill="currentColor" stroke="none" />
      <circle cx="3" cy="12" r="0.9" fill="currentColor" stroke="none" />
    </template>
    <template v-else-if="name === 'ol'">
      <path d="M7 4h6M7 8h6M7 12h6" />
      <text x="1.2" y="5.6" font-size="5" font-weight="500" fill="currentColor" stroke="none">1</text>
      <text x="1.2" y="9.6" font-size="5" font-weight="500" fill="currentColor" stroke="none">2</text>
      <text x="1.2" y="13.6" font-size="5" font-weight="500" fill="currentColor" stroke="none">3</text>
    </template>
    <template v-else-if="name === 'task'">
      <rect x="2" y="3.5" width="9.5" height="9.5" rx="1.8" />
      <path d="M4.4 8.2 6.6 10.4l5.4-5.6" />
    </template>
  </svg>
</template>
