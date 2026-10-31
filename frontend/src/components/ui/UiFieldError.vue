<script setup lang="ts">
import { computed } from 'vue'

/**
 * 字段错误（#2156 表单批）：校验失败时的一行（或多行）红字。
 *
 * 支持一次显示多条：传 `errors` 数组，去重后单条直出、多条渲染列表。
 * 改前只能传一个字符串，页面要么只显示第一条，要么自己拼成一长串。
 *
 * `role="alert"` 保证错误出现时读屏会播报；与控件的语义关联由调用方用
 * `aria-describedby` 指向本节点 id（或由页面把错误 id 传给控件）。
 */
const props = defineProps<{
  /** 错误文案集合；空值 / 空字符串会被过滤 */
  errors?: Array<string | undefined | null>
}>()

const messages = computed(() => [...new Set((props.errors ?? []).filter((m): m is string => Boolean(m)))])
</script>

<template>
  <div
    v-if="$slots.default || messages.length"
    data-slot="field-error"
    role="alert"
    class="m-0 text-xs leading-normal text-bad"
  >
    <slot>
      <ul v-if="messages.length > 1" class="m-0 flex list-disc flex-col gap-1 pl-4">
        <li v-for="message in messages" :key="message">{{ message }}</li>
      </ul>
      <template v-else>{{ messages[0] }}</template>
    </slot>
  </div>
</template>
