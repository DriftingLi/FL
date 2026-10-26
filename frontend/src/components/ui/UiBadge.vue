<script setup lang="ts">
/**
 * 角标：数字提示（未读数、待办数）与纯状态点。
 * 数量超过 max 显示为 `max+`。
 *
 * 两种用法（#1977 补第二种）：
 * - **独立**（不传 slot）：自己就是那个小圆点 / 药丸，跟在文字后面；
 * - **附着**（传 slot）：压在被包裹内容（图标、头像、页签）的右上角。
 */
const props = withDefaults(
  defineProps<{
    count?: number
    tone?: 'brand' | 'ok' | 'warn' | 'bad' | 'neutral'
    /** 只显示圆点，忽略 count */
    dot?: boolean
    max?: number
  }>(),
  { count: 0, tone: 'brand', dot: false, max: 99 }
)

const TONE: Record<string, string> = {
  brand: 'bg-ui-600',
  ok: 'bg-ok-fill',
  warn: 'bg-warn-fill',
  bad: 'bg-bad-fill',
  neutral: 'bg-ink-3'
}

/** 计数文字与填充成对（#1895）：浅色深底配白字、深色亮底配深字，两主题都 ≥4.5 */
const TONE_TEXT: Record<string, string> = {
  brand: 'text-white',
  ok: 'text-ok-on',
  warn: 'text-warn-on',
  bad: 'text-bad-on',
  neutral: 'text-white'
}

const badgeText = (n: number, max: number) => (n > max ? max + '+' : String(n))
</script>

<template>
  <!-- 附着：压在被包裹内容的右上角 -->
  <span v-if="$slots.default" class="relative inline-flex">
    <slot />
    <span
      v-if="props.dot"
      class="absolute -right-0.5 -top-0.5 h-2 w-2 rounded-pill"
      :class="TONE[props.tone]"
    />
    <span
      v-else-if="props.count > 0"
      class="absolute -right-2 -top-1.5 inline-flex min-w-[18px] items-center justify-center rounded-pill px-1.5 py-0.5 text-xs font-medium leading-none"
      :class="[TONE[props.tone], TONE_TEXT[props.tone]]"
    >
      {{ badgeText(props.count, props.max) }}
    </span>
  </span>

  <!-- 独立 -->
  <span
    v-else-if="props.dot"
    class="inline-block h-2 w-2 shrink-0 rounded-pill"
    :class="TONE[props.tone]"
  />
  <span
    v-else-if="props.count > 0"
    class="inline-flex min-w-[18px] items-center justify-center rounded-pill px-1.5 py-0.5 text-xs font-medium leading-none"
    :class="[TONE[props.tone], TONE_TEXT[props.tone]]"
  >
    {{ badgeText(props.count, props.max) }}
  </span>
</template>
