<template>
  <!-- 诊断来源资料（ADR-0033）：默认折叠，图片懒加载；R1 原子类区：模板只用原子类 -->
  <div>
    <div
      v-if="!embedded"
      class="flex cursor-pointer items-center justify-between gap-2 py-0.5 text-xs text-ink-3"
      @click="open = !open"
    >
      <span>▸ 资料来源（{{ sources.length }} 条）</span>
      <span class="whitespace-nowrap text-ui-600">{{ open ? '收起 ▴' : '展开 ▾' }}</span>
    </div>
    <template v-if="embedded || open">
      <div
        v-for="source in sources"
        :key="source.id"
        class="mt-2.5 rounded-[10px] border border-line bg-panel px-3 py-2.5"
      >
        <img
          v-for="imagePath in sourceImages(source.text)"
          :key="imagePath"
          :src="aiAssistantApi.manualUrl(imagePath)"
          class="mb-1.5 block max-h-[220px] max-w-full rounded-ctl border border-line"
          alt="资料图片"
          loading="lazy"
        />
        <div class="source-text text-[13px] leading-[1.6] text-ink-2">
          {{ stripImageMarkers(source.text) }}
        </div>
        <div class="mt-1.5 flex items-center gap-3 text-xs text-ink-3">
          <span v-if="source.metadata?.page_start">第 {{ source.metadata.page_start }}{{ source.metadata.page_end && source.metadata.page_end !== source.metadata.page_start ? '-' + source.metadata.page_end : '' }} 页</span>
          <a
            v-if="source.metadata?.source_url"
            :href="source.metadata.source_url"
            target="_blank"
            rel="noopener"
            class="text-ui-600 no-underline hover:underline"
          >打开 PDF 来源</a>
          <span v-if="!source.metadata?.source_url && !source.metadata?.page_start">暂无来源资料</span>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// 诊断来源资料展示（逐轮回放 + 当轮面板共用）：IMAGE 标记解析与前缀 strip 单点。
// 外部助手吐绝对路径（/assistant/static/manual/…，lxc101 取证）：strip 后再进后端代理。
import { ref } from 'vue'
import { aiAssistantApi, type DiagnosisSource } from '@/api/aiAssistant'

withDefaults(
  defineProps<{
    sources: DiagnosisSource[]
    /** 当轮面板复用：不渲染自己的折叠头，由外层控制展开 */
    embedded?: boolean
  }>(),
  { embedded: false }
)

const open = ref(false)

function stripAssistantPrefix(path: string): string {
  return path.replace(/^\/assistant\/static\//, '').replace(/^assistant\/static\//, '')
}

function sourceImages(text: string): string[] {
  const out: string[] = []
  const re = /<<IMAGE:([^>]+)>>/g
  let m: RegExpExecArray | null
  while ((m = re.exec(text))) {
    const path = stripAssistantPrefix(m[1].trim())
    if (path) out.push(path)
  }
  return out
}

function stripImageMarkers(text: string): string {
  return text.replace(/<<IMAGE:[^>]+>>/g, '').trim()
}
</script>

<style scoped>
/* R1 允许保留：多行截断的 mask 渐隐（原子类无等价物）。其余已全部原子化。 */
.source-text {
  white-space: pre-wrap;
  max-height: 120px;
  overflow: hidden;
  mask-image: linear-gradient(to bottom, #000 70%, transparent);
  -webkit-mask-image: linear-gradient(to bottom, #000 70%, transparent);
}
</style>
