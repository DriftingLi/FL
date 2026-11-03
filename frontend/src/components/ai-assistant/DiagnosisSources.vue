<template>
  <!-- 诊断来源资料（ADR-0033）：默认折叠。R1 原子类区：模板只用原子类。
       v1 通道退役后（票 2237）这里**只回放文字**：<<IMAGE:…>> 标记指向的手册/案例资源
       已随旧代理一起下线，历史行里的图不再可达，故标记一律剥掉而不是渲染成坏图。 -->
  <div>
    <SourcesFoldHeader
      v-if="!embedded"
      title="资料来源"
      :count="sources.length"
      :open="open"
      @toggle="open = !open"
    />
    <UiCollapse :open="embedded || open" lazy>
      <div
        v-for="(source, si) in sources"
        :key="source.id ?? si"
        class="mt-2.5 rounded-ctl border border-line bg-panel px-3 py-2.5 text-sm leading-[1.6] text-ink-2"
      >
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
          <span v-if="!source.metadata?.source_url && !source.metadata?.page_start">暂无来源资料</span>
        </div>
      </div>
    </UiCollapse>
  </div>
</template>

<script setup lang="ts">
// 诊断来源资料展示（逐轮回放 + 当轮面板共用）：IMAGE 标记剥离单点。
//
// v1 代理退役（票 2237）后本组件不再产出任何 URL：
//   - 溯源图 <<IMAGE:/assistant/static/manual/…>> 的上游已下线，渲染即坏图；
//   - PDF 原文外链是助手内网路径，同一原因不可达。
// v2 的回答配图**在正文里**（adapter 已归一成 /diagnosis/ossimg/* 站内代理），不经本组件。
import { ref } from 'vue'
import MarkdownRender from 'markstream-vue'
import UiCollapse from '@/components/ui/UiCollapse.vue'
import { MARKSTREAM_MERMAID_PROPS } from '@/utils/markstreamRuntime'
import type { DiagnosisSource } from '@/api/aiAssistant'
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

const IMAGE_RE = /<<IMAGE:([^>]+)>>/g

function stripImageMarkers(text: string): string {
  IMAGE_RE.lastIndex = 0
  return text.replace(IMAGE_RE, '').trim()
}
</script>
