<script setup lang="ts">
import type { IconName } from '@/icons/registry'
import UiIcon from '@/components/ui/UiIcon.vue'
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
    icon?: IconName | 'like' | 'fav' | 'report' | 'delete'
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
    /**
     * 无边框变体（ADR-0042 回复卡底部操作行）：去掉描边与边框类 hover，只留图标+计数。
     * 默认 false = 描边药丸（现状），未传值的既有调用方零 diff。
     */
    borderless?: boolean
  }>(),
  {
    tone: 'neutral',
    active: false,
    compact: false,
    disabled: false,
    borderless: false
  }
)

const emit = defineEmits<{ (e: 'click', ev: MouseEvent): void }>()

/** 内置字形："描边 ↔ 实心"成对登记在注册表里（chip-* / chip-*-filled，#1913） */
const CHIP_ICONS = ['like', 'fav', 'report', 'delete'] as const
type ChipIcon = (typeof CHIP_ICONS)[number]
const CHIP_ICON_SET: Readonly<Record<string, true>> = { like: true, fav: true, report: true, delete: true }

const builtin = computed<ChipIcon | null>(() => {
  const icon = props.icon
  if (typeof icon !== 'string') return null
  if (!(icon in CHIP_ICON_SET)) return null
  return icon as ChipIcon
})

/** 交给 UiIcon 的名字：内置字形按激活态切 -filled，外部图标原样透传 */
const chipIcon = computed<IconName | null>(() => {
  if (builtin.value) return ('chip-' + builtin.value + (props.active ? '-filled' : '')) as IconName
  return (props.icon as IconName | undefined) ?? null
})

/**
 * 只有「形状真的变了」才走形变：心动两态是同一个 d（只换填充），形变无处可动，保持 CSS；
 * 星形/旗/桶的描边与实心是两套路径 —— 正是形变的宿主（ADR-0079 白名单）。
 */
const chipMorph = computed(() => builtin.value !== null && builtin.value !== 'like')

const chipClass = computed(() => {
  const active = props.active
  // 无边框变体：激活态只留语义色（无底色），未激活只留文字色变化 —— 不产出任何 border-* 类。
  if (props.borderless) {
    if (active && props.tone === 'like') return 'text-rose'
    if (active && props.tone === 'fav') return 'text-warn'
    if (props.tone === 'danger') return 'text-ink-3 hover:text-bad-strong'
    if (props.tone === 'like') return 'text-ink-3 hover:text-rose'
    if (props.tone === 'fav') return 'text-ink-3 hover:text-warn'
    return 'text-ink-3 hover:text-ink-2'
  }
  if (active && props.tone === 'like') return 'bg-rose-soft text-rose border-transparent'
  if (active && props.tone === 'fav') return 'bg-warn-soft text-warn border-transparent'
  if (props.tone === 'danger') return 'text-ink-3 hover:text-bad-strong hover:bg-bad-soft hover:border-transparent'
  if (props.tone === 'like') return 'text-ink-3 hover:text-rose hover:border-rose/40'
  if (props.tone === 'fav') return 'text-ink-3 hover:text-warn hover:border-warn/40'
  return 'text-ink-3 hover:text-ink-2 hover:border-line-strong'
})

/** 基础类：无边框变体去掉 border 与药丸内边距（图标+计数紧贴，照截图的操作行形态） */
const baseClass = computed(() =>
  props.borderless
    ? 'inline-flex cursor-pointer items-center gap-1 rounded-ctl border-0 bg-transparent px-1.5 py-1 text-xs font-medium leading-none transition-colors duration-150 disabled:pointer-events-none disabled:opacity-50'
    : 'inline-flex cursor-pointer items-center gap-1.5 rounded-pill border px-2.5 py-1.5 text-xs font-medium leading-none transition-all duration-150 hover:-translate-y-px disabled:pointer-events-none disabled:opacity-50'
)
</script>

<template>
  <button
    type="button"
    :disabled="disabled"
    :class="[baseClass, chipClass]"
    @click="(ev: MouseEvent) => emit('click', ev)"
  >
    <UiIcon v-if="chipIcon" :name="chipIcon" :morph="chipMorph" class="size-3.5 flex-none text-[1em]" />
    <span v-if="!compact && label">{{ label }}</span>
    <span v-if="count !== undefined && count > 0" class="font-mono tabular-nums">{{ count }}</span>
    <span v-if="compact && label" class="sr-only">{{ label }}</span>
  </button>
</template>
