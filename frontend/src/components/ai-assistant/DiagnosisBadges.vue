<template>
  <!-- 能力徽章行（票 2245 的 badges 词表，2244 渲染）：答案**上方**一排小胶囊。
       对方同一位置（`app.js:265-271` 的 `.caps`）也是纯展示、不可点；这里同样不加交互。
       emoji 与 cls 都是 v2 原样透出的，cls 只作视觉提示，不参与语义判定 ——
       未知 cls（对方加词）不该让徽章消失或变形，故样式只取中性档。 -->
  <div v-if="badges.length" class="mb-2 flex flex-wrap items-center gap-1.5">
    <span
      v-for="b in badges"
      :key="b.cls || b.name"
      class="inline-flex shrink-0 items-center gap-1 rounded-pill border border-line bg-canvas px-2 py-0.5 text-xs leading-[1.5] text-ink-2"
      :data-cap="b.cls || undefined"
    >
      <span v-if="b.emoji" aria-hidden="true">{{ b.emoji }}</span>{{ b.name }}
    </span>
  </div>
</template>

<script setup lang="ts">
// 徽章渲染：三字段（name / emoji / cls）全部来自后端 DTO，这里不维护词表副本 ——
// v2 的九词表在对方侧，加词不需要改前端。data-cap 留给将来要按类别分档时用。
import type { DiagnosisBadge } from '@/api/aiAssistant'

withDefaults(
  defineProps<{
    /** 该条消息的徽章；空数组时不渲染任何节点 */
    badges?: DiagnosisBadge[]
  }>(),
  { badges: () => [] }
)
</script>
