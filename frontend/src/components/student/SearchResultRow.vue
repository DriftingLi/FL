<template>
  <div class="flex flex-col">
    <div class="flex items-baseline gap-2.5">
      <span class="max-w-[50%] shrink-0 truncate text-sm text-ink">
        <template v-for="(seg, i) in titleSegments" :key="'t' + i">
          <mark v-if="seg.hit" class="bg-transparent font-semibold text-brand">{{ seg.text }}</mark>
          <template v-else>{{ seg.text }}</template>
        </template>
      </span>
      <span class="truncate text-[13px] text-ink-3">
        <template v-for="(seg, i) in snippetSegments" :key="'s' + i">
          <mark v-if="seg.hit" class="bg-transparent font-semibold text-ink-2">{{ seg.text }}</mark>
          <template v-else>{{ seg.text }}</template>
        </template>
      </span>
    </div>
    <div v-if="hitLabel" class="mt-0.5 text-[12px] text-ink-muted">{{ hitLabel }}</div>
  </div>
</template>

<script setup lang="ts">
// 搜索结果行（ADR-0049 决策 6）：
// 后端给的是**源串命中窗口**，投影与高亮在端上做 —— 投影走 ADR-0044 的 markdownToPlainText
// （与渲染同源的解析器投影，不是正则剥标记），高亮在投影后的纯文本上重新定位关键词。
import { computed } from 'vue'
import { markdownToPlainText } from '@/utils/markdownText'
import type { SearchItem } from '@/api/search'

const props = defineProps<{ item: SearchItem; keyword: string }>()

interface Segment {
  text: string
  hit: boolean
}

function split(text: string, keyword: string): Segment[] {
  const plain = text ?? ''
  const kw = keyword.trim()
  if (!plain) return []
  if (!kw) return [{ text: plain, hit: false }]
  const lower = plain.toLowerCase()
  const needle = kw.toLowerCase()
  const out: Segment[] = []
  let cursor = 0
  for (;;) {
    const idx = lower.indexOf(needle, cursor)
    if (idx < 0) break
    if (idx > cursor) out.push({ text: plain.slice(cursor, idx), hit: false })
    out.push({ text: plain.slice(idx, idx + needle.length), hit: true })
    cursor = idx + needle.length
  }
  if (cursor < plain.length) out.push({ text: plain.slice(cursor), hit: false })
  return out
}

const snippetText = computed(() => markdownToPlainText(props.item.snippet || props.item.summary || ''))
const titleSegments = computed(() => split(props.item.title ?? '', props.keyword))
const snippetSegments = computed(() => split(snippetText.value, props.keyword))

/** 命中位置标注：论坛主题的命中落在回复里时必须说清，否则点进去找不到关键词 */
const hitLabel = computed(() => (props.item.hit_field === 'reply' ? '命中在回复' : ''))
</script>
