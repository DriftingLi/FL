<template>
  <!-- 诊断来源资料（ADR-0033）：默认折叠，图片懒加载；R1 原子类区：模板只用原子类 -->
  <div>
    <SourcesFoldHeader
      v-if="!embedded"
      title="资料来源"
      :count="sources.length"
      :open="open"
      @toggle="open = !open"
    />
    <template v-if="embedded || open">
      <div
        v-for="(source, si) in sources"
        :key="source.id ?? si"
        class="mt-2.5 rounded-[10px] border border-line bg-panel px-3 py-2.5 text-[13px] leading-[1.6] text-ink-2"
      >
        <img
          v-for="imagePath in sourceImages(source.text)"
          :key="imagePath"
          :src="aiAssistantApi.manualUrl(imagePath)"
          class="mb-1.5 block max-h-[220px] w-auto max-w-full rounded-ctl border border-line"
          alt="资料图片"
          loading="lazy"
        />
        <MarkdownRender
          mode="chat"
          :content="stripImageMarkers(source.text)"
          :final="true"
          html-policy="escape"
          :mermaid-props="MARKSTREAM_MERMAID_PROPS"
          :fade="false"
          class="line-clamp-6"
        />
        <div class="mt-1.5 flex items-center gap-3 text-xs text-ink-3">
          <span v-if="source.metadata?.page_start">第 {{ source.metadata.page_start }}{{ source.metadata.page_end && source.metadata.page_end !== source.metadata.page_start ? '-' + source.metadata.page_end : '' }} 页</span>
          <a
            v-if="pdfProxyUrl(source)"
            :href="pdfProxyUrl(source)"
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
import MarkdownRender from 'markstream-vue'
import { MARKSTREAM_MERMAID_PROPS } from '@/utils/markstreamRuntime'
import { aiAssistantApi, type DiagnosisSource } from '@/api/aiAssistant'
import SourcesFoldHeader from '@/components/ai-assistant/SourcesFoldHeader.vue'

withDefaults(
  defineProps<{
    sources: DiagnosisSource[]
    /** 当轮面板复用：不渲染自己的折叠头，由外层控制展开 */
    embedded?: boolean
  }>(),
  { embedded: false }
)

const open = ref(false)

const ASSISTANT_STATIC_RE = /^\/?(?:assistant\/static\/)?(?:manual\/)?/

// lxc101 取证：IMAGE 标记为 /assistant/static/manual/<doc>/page_N.png（manual/ 段是
// 目录名的一部分），strip 后交给后端拼 /assistant/static/manual/ 前缀。
function stripAssistantPrefix(path: string): string {
  return path.replace(ASSISTANT_STATIC_RE, '')
}

const IMAGE_RE = /<<IMAGE:([^>]+)>>/g

// PDF 原文外链是助手内网路径（/assistant/static/manual/…pdf），公网不可达且登录态
// 不同域——统一走本站手册代理，#page 锚点保留。
function pdfProxyUrl(source: DiagnosisSource): string | undefined {
  const url = source.metadata?.source_url
  if (!url) return undefined
  const m = url.match(/\/assistant\/static\/manual\/(.+?\.pdf)/i)
  return m ? aiAssistantApi.manualUrl(m[1]) : url
}

function sourceImages(text: string): string[] {
  const out: string[] = []
  IMAGE_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = IMAGE_RE.exec(text))) {
    const path = stripAssistantPrefix(m[1].trim())
    if (path) out.push(path)
  }
  return out
}

function stripImageMarkers(text: string): string {
  IMAGE_RE.lastIndex = 0
  return text.replace(IMAGE_RE, '').trim()
}
</script>
