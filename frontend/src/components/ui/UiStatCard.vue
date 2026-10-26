<script setup lang="ts">
import type { IconName } from '@/icons/registry'
import UiIcon from '@/components/ui/UiIcon.vue'
import { computed } from 'vue'
import UiCountTo from './UiCountTo.vue'

/**
 * 统计卡：自写。Dashboard 与各类概览页的指标块，全站唯一形态（ADR-0087）。
 *
 * 数值用 tabular-nums 保证等宽，避免刷新时数字跳动导致宽度抖动。
 * trend 为正显示上升、为负显示下降；reduced-motion 由 global.css 统一压平。
 */
const props = withDefaults(
  defineProps<{
    label: string
    value?: number | string
    unit?: string
    /** 环比变化，正=上升 负=下降；不传则不显示 */
    trend?: number
    icon?: IconName
    tone?: 'brand' | 'ok' | 'warn' | 'bad' | 'neutral'
    loading?: boolean
    /**
     * 数值滚动进场。仅当 value 是数字时生效 —— 字符串（如「3/12」）无法插值。
     * 默认关闭：静态数字在密集列表里反而更稳。
     */
    countTo?: boolean
    /**
     * 版式档（ADR-0087 决策 2）。三档都是**同一意图的版式**，不是三个组件：
     * - `icon-right`：标签在上、数值在下，图标块靠右（学员端现状，默认）
     * - `icon-left`：图标块靠左，数值在上、标签在下（管理端仪表盘现状）
     * - `plain`：居中、无图标，数值用 tone 的墨色（管理端统计分析现状）
     *
     * 默认值 = 本组件改造前的外观，未传值的调用方零 diff（ADR-0035 决策 4 / R2）。
     */
    layout?: 'icon-right' | 'icon-left' | 'plain'
    /** 左侧 4px 色条（仪表盘现状）；颜色跟 tone 走，不另传色值 */
    accent?: boolean
    /** 悬停抬升：阴影 xs→md + 上浮 2px（仪表盘现状） */
    hoverable?: boolean
  }>(),
  {
    tone: 'brand',
    loading: false,
    countTo: false,
    layout: 'icon-right',
    accent: false,
    hoverable: false
  }
)

/** 只有数字才走 UiCountTo；非数字与关闭时行为与改造前一致 */
const numericValue = computed(() => {
  if (!props.countTo) return null
  return typeof props.value === 'number' && Number.isFinite(props.value) ? props.value : null
})

/** 图标块：浅底 + 深字，随主题翻转 */
const TONE: Record<string, string> = {
  brand: 'bg-ui-soft text-ui-soft-ink',
  ok: 'bg-ok-soft text-ok',
  warn: 'bg-warn-soft text-warn',
  bad: 'bg-bad-soft text-bad',
  neutral: 'bg-canvas text-ink-2'
}

/** 纯数字版式（plain）的数值墨色：与图标块同源，只是换成前景 */
const TONE_INK: Record<string, string> = {
  brand: 'text-ui-soft-ink',
  ok: 'text-ok',
  warn: 'text-warn',
  bad: 'text-bad',
  neutral: 'text-ink-3'
}

/** 色条：实心档，取各 tone 的强色 */
const TONE_BAR: Record<string, string> = {
  brand: 'bg-ui-600',
  ok: 'bg-ok',
  warn: 'bg-warn',
  bad: 'bg-bad',
  neutral: 'bg-ink-3'
}
</script>

<template>
  <div
    class="relative overflow-hidden rounded-card border border-line bg-panel shadow-card"
    :class="[
      props.hoverable
        ? 'transition-[box-shadow,transform] duration-[var(--duration-normal)] ease-[var(--ease-default)] hover:-translate-y-0.5 hover:shadow-raised'
        : '',
      props.layout === 'plain' ? 'text-center' : ''
    ]"
  >
    <span
      v-if="props.accent"
      class="absolute left-0 top-0 h-full w-1"
      :class="TONE_BAR[props.tone]"
      aria-hidden="true"
    />

    <div
      class="flex gap-4 p-5"
      :class="[
        props.layout === 'icon-left' ? 'items-center' : 'items-start',
        props.layout === 'icon-right' ? 'flex-row-reverse justify-between' : ''
      ]"
    >
      <span
        v-if="props.icon && props.layout !== 'plain'"
        class="grid shrink-0 place-items-center"
        :class="[
          props.layout === 'icon-left' ? 'h-11 w-11 rounded-card' : 'h-9 w-9 rounded-ctl',
          TONE[props.tone]
        ]"
      >
        <UiIcon :name="props.icon" :size="props.layout === 'icon-left' ? 22 : undefined" />
      </span>

      <div class="min-w-0 flex-1">
        <p v-if="props.layout === 'icon-right'" class="text-xs text-ink-3">{{ props.label }}</p>

        <div
          v-if="props.loading"
          class="mt-2 h-7 w-16 animate-pulse rounded-ctl bg-canvas"
          :class="props.layout === 'plain' ? 'mx-auto' : ''"
        />
        <p
          v-else
          class="flex items-baseline gap-1"
          :class="[
            props.layout === 'icon-right' ? 'mt-1' : '',
            props.layout === 'plain' ? 'justify-center' : ''
          ]"
        >
          <span
            class="font-heading tabular-nums"
            :class="props.layout === 'plain'
              ? ['text-3xl font-bold leading-[1.3]', TONE_INK[props.tone]]
              : ['text-2xl font-semibold leading-[1.2] text-ink']"
          >
            <UiCountTo v-if="numericValue !== null" :to="numericValue" />
            <span v-else>{{ props.value ?? '—' }}</span>
          </span>
          <span v-if="props.unit" class="text-xs text-ink-3">{{ props.unit }}</span>
        </p>

        <p
          v-if="props.trend !== undefined"
          class="mt-1 text-xs"
          :class="props.trend >= 0 ? 'text-ok' : 'text-bad'"
        >
          {{ props.trend >= 0 ? '↑' : '↓' }} {{ Math.abs(props.trend) }}%
        </p>

        <p v-if="props.layout !== 'icon-right'" class="mt-0.5 text-sm text-ink-3">
          {{ props.label }}
        </p>
      </div>
    </div>

    <slot />
  </div>
</template>
