<template>
  <!-- 诊断来源资料（ADR-0033）：默认折叠，图片懒加载；bare 复用当轮面板的卡片样式 -->
  <div class="diagnosis-sources-block">
    <div
      v-if="!bare"
      class="sources-head"
      @click="open = !open"
    >
      <span>▸ 资料来源（{{ sources.length }} 条）</span>
      <span class="sources-toggle">{{ open ? '收起 ▴' : '展开 ▾' }}</span>
    </div>
    <template v-if="bare || open">
      <div
        v-for="source in sources"
        :key="source.id"
        class="source-card"
      >
        <img
          v-for="imagePath in sourceImages(source.text)"
          :key="imagePath"
          :src="aiAssistantApi.manualUrl(imagePath)"
          class="source-image"
          alt="资料图片"
          loading="lazy"
        />
        <div class="source-text">
          {{ stripImageMarkers(source.text) }}
        </div>
        <div class="source-meta">
          <span v-if="source.metadata?.page_start">第 {{ source.metadata.page_start }}{{ source.metadata.page_end && source.metadata.page_end !== source.metadata.page_start ? '-' + source.metadata.page_end : '' }} 页</span>
          <a
            v-if="source.metadata?.source_url"
            :href="source.metadata.source_url"
            target="_blank"
            rel="noopener"
            class="source-link"
          >打开 PDF 来源</a>
          <span v-if="!source.metadata?.source_url && !source.metadata?.page_start" class="source-empty">暂无来源资料</span>
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
    bare?: boolean
  }>(),
  { bare: false }
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
/* 来源卡片样式沿用 FeatureChatPage 既有 token（R1 原子类区外的新组件，scoped 仅做本组件结构） */
.sources-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  cursor: pointer;
  padding: 2px 0;
}

.sources-toggle {
  color: var(--color-primary-600);
  white-space: nowrap;
}

.source-card {
  border: 1px solid var(--color-border-light);
  border-radius: 10px;
  padding: 10px 12px;
  background: var(--color-bg-card);
  margin-top: 10px;
}

.source-image {
  max-width: 100%;
  max-height: 220px;
  border-radius: 8px;
  margin-bottom: 6px;
  display: block;
  border: 1px solid var(--color-border-light);
}

.source-text {
  font-size: 13px;
  color: var(--color-text-secondary);
  white-space: pre-wrap;
  line-height: 1.6;
  max-height: 120px;
  overflow: hidden;
  mask-image: linear-gradient(to bottom, #000 70%, transparent);
  -webkit-mask-image: linear-gradient(to bottom, #000 70%, transparent);
}

.source-meta {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 6px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.source-link {
  color: var(--color-primary-600);
  text-decoration: none;
}

.source-link:hover {
  text-decoration: underline;
}

.source-empty {
  color: var(--color-text-tertiary);
}
</style>
