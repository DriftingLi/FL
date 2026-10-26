<template>
  <!-- 小胶囊（方案 B 工具栏）：图标 + 文字 + 可选角标，单行不换行 -->
  <button
    type="button"
    class="flex shrink-0 cursor-pointer items-center gap-1.5 whitespace-nowrap rounded-pill border border-line bg-panel px-3.5 py-1.5 text-[13px] text-ink-2 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:bg-ui-soft hover:text-ui-soft-ink"
    :class="active ? 'border-ui-soft-ink bg-ui-soft font-semibold text-ui-soft-ink' : ''"
    @click="emit('click', $event)"
  >
    <UiIcon :name="icon" v-if="icon" :size="14" />
    <slot>{{ label }}</slot>
    <i v-if="badge" class="bg-ok-fill text-ok-on rounded-pill px-1.5 py-px text-[10px] font-semibold not-italic leading-[1.4]">{{ badge }}</i>
  </button>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
// 小胶囊：AI 助手主界面功能入口与诊断筛选/快捷问的共用形态（消 twin class 串重复）。
import type { IconName } from '@/icons/registry'

const props = withDefaults(
  defineProps<{
    label?: string
    /** 注册表图标名 */
    icon?: IconName
    badge?: string
    active?: boolean
  }>(),
  { label: '', badge: '', active: false }
)

const emit = defineEmits<{
  (e: 'click', ev: MouseEvent): void
}>()

</script>
