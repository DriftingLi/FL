<script setup lang="ts">
import { computed } from 'vue'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/utils/cn'

/**
 * 角标：数字提示（未读数、待办数）与纯状态点。
 * 数量超过 max 显示为 `max+`。
 *
 * 两种用法（#1977 补第二种）：
 * - **独立**（不传 slot）：自己就是那个小圆点 / 药丸，跟在文字后面；
 * - **附着**（传 slot）：压在被包裹内容（图标、头像、页签）的右上角。
 *
 * 变体从两张 `Record<string, string>` 收成一份 `cva` 定义（#2156 变体批）：
 * tone 决定配色、shape 决定药丸还是点，二者正交；同属性冲突由 `cn()` 合并。
 * 配色成对（#1895）：浅色深底配白字、深色亮底配深字，两主题都 ≥4.5。
 */
const badgeVariants = cva(
  'inline-flex items-center justify-center rounded-pill text-xs font-medium leading-none whitespace-nowrap',
  {
    variants: {
      tone: {
        brand: 'bg-ui-600 text-white',
        ok: 'bg-ok-fill text-ok-on',
        warn: 'bg-warn-fill text-warn-on',
        bad: 'bg-bad-fill text-bad-on',
        neutral: 'bg-ink-3 text-white'
      },
      shape: {
        pill: 'min-w-[18px] px-1.5 py-0.5',
        dot: 'size-2'
      }
    },
    defaultVariants: { tone: 'brand', shape: 'pill' }
  }
)

/** tone 的类型导出：调用方的映射表可以用它收窄（`Record<string, UiBadgeTone>`） */
export type UiBadgeTone = NonNullable<VariantProps<typeof badgeVariants>['tone']>

const props = withDefaults(
  defineProps<{
    count?: number
    tone?: UiBadgeTone
    /** 只显示圆点，忽略 count */
    dot?: boolean
    max?: number
  }>(),
  { count: 0, tone: 'brand', dot: false, max: 99 }
)

const badgeClass = computed(() => cn(badgeVariants({ tone: props.tone, shape: props.dot ? 'dot' : 'pill' })))

const badgeText = (n: number, max: number) => (n > max ? max + '+' : String(n))
</script>

<template>
  <!-- 附着：压在被包裹内容的右上角 -->
  <span v-if="$slots.default" class="relative inline-flex">
    <slot />
    <span
      v-if="props.dot"
      :class="cn(badgeClass, 'absolute -top-0.5 -right-0.5')"
    />
    <span
      v-else-if="props.count > 0"
      :class="cn(badgeClass, 'absolute -top-1.5 -right-2')"
    >
      {{ badgeText(props.count, props.max) }}
    </span>
  </span>

  <!-- 独立 -->
  <span v-else-if="props.dot" :class="badgeClass" />
  <span v-else-if="props.count > 0" :class="badgeClass">
    {{ badgeText(props.count, props.max) }}
  </span>
</template>
