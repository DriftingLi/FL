<script setup lang="ts">
/**
 * 操作药丸：图标 + 文字 + 计数的圆角按钮，统一点赞/收藏/举报/删除等行内互动。
 *
 * tone 语义：
 * - like / fav（互动类）：未激活中性描边，激活态语义色 soft 底 + 主色 + 图标实心
 * - neutral（治理类，如举报）：常驻中性，hover 微深
 * - danger（危险类，如删除）：常驻中性，hover 才显红（危险语义不常驻噪音）
 *
 * icon：内置语义图标（like 心形 / fav 星形 / report 旗 / delete 筒），无需外部依赖；
 * 激活态自动由 outline 切 filled。也可传 EP 图标引用覆盖（icon 具名组件优先）。
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    /** 图标：内置 key 或组件引用/全局名（内置 key 优先，激活可切实心） */
    icon?: string | object
    /** 主文字 */
    label?: string
    /** 计数（tabular-nums） */
    count?: number
    /** 语义色调（决定激活填充色） */
    tone?: 'like' | 'fav' | 'neutral' | 'danger'
    /** 激活态 */
    active?: boolean
    /** 紧凑形态：仅图标 + 计数 */
    compact?: boolean
    disabled?: boolean
  }>(),
  {
    tone: 'neutral',
    active: false,
    compact: false,
    disabled: false
  }
)

const emit = defineEmits<{ (e: 'click', ev: MouseEvent): void }>()

/** 内置图标 path（outline → filled 双形态，激活自动切换） */
const PATHS: Record<string, { outline: string; filled: string }> = {
  like: {
    outline: 'M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z',
    filled: 'M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z'
  },
  fav: {
    outline: 'M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z',
    filled: 'M12 17.27 18.18 21l-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z'
  },
  report: {
    outline: 'M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1zM4 22v-7',
    filled: 'M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1zm0 7v-7'
  },
  delete: {
    outline: 'M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6',
    filled: 'M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m4 3v9m6-9v9'
  }
}

const builtin = computed(() => (typeof props.icon === 'string' && props.icon in PATHS ? props.icon as keyof typeof PATHS : null))
const path = computed(() => (builtin.value ? PATHS[builtin.value][props.active ? 'filled' : 'outline'] : ''))

const chipClass = computed(() => {
  const active = props.active
  if (active && props.tone === 'like') return 'bg-rose-soft text-rose border-transparent'
  if (active && props.tone === 'fav') return 'bg-warn-soft text-warn border-transparent'
  if (props.tone === 'danger') return 'text-ink-3 hover:text-bad-strong hover:bg-bad-soft hover:border-transparent'
  if (props.tone === 'like') return 'text-ink-3 hover:text-rose hover:border-rose/40'
  if (props.tone === 'fav') return 'text-ink-3 hover:text-warn hover:border-warn/40'
  return 'text-ink-3 hover:text-ink-2 hover:border-line-strong'
})
</script>

<template>
  <button
    type="button"
    :disabled="disabled"
    class="inline-flex cursor-pointer items-center gap-1.5 rounded-pill border px-2.5 py-1.5 text-xs font-medium leading-none transition-all duration-150 hover:-translate-y-px disabled:pointer-events-none disabled:opacity-50"
    :class="chipClass"
    @click="(ev: MouseEvent) => emit('click', ev)"
  >
    <!-- 内置 SVG（激活切 filled；stroke 线型 → fill 实心） -->
    <svg
      v-if="builtin"
      class="size-3.5 flex-none"
      viewBox="0 0 24 24"
      :fill="active ? 'currentColor' : 'none'"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      :aria-hidden="label ? undefined : 'true'"
    >
      <path :d="path" />
    </svg>
    <el-icon v-else-if="icon" class="size-3.5 text-[1em]"><component :is="icon" /></el-icon>
    <span v-if="!compact && label">{{ label }}</span>
    <span v-if="count !== undefined && count > 0" class="font-mono tabular-nums">{{ count }}</span>
    <span v-if="compact && label" class="sr-only">{{ label }}</span>
  </button>
</template>
