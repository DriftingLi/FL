<script setup lang="ts">
/**
 * 「发布端预览」弹窗（ADR-0046 / #903 / #2631）。
 *
 * 讲师端章节编辑、管理端内容精选编辑、管理端 AI 生成预览三处共用**同一个壳**：
 * 内容由调用方给（编辑器最新值的快照），渲染固定走发布端单点 PublishMarkdown，
 * 不发起任何请求、不写回任何表单。
 *
 * 抽成组件的理由不是省几行：预览一旦各写一套，就会出现「这个入口的预览走 marked、
 * 那个入口又接回 v-html」——那正是这批要消灭的东西。壳收在这里，调用方只能选档位（subset）。
 *
 * 两种正文形态（#2631）：`docxUrl` 有值时改渲染原始 Word（DocxDocument），
 * 这样管理端在「导入 Word」形态下也能点开预览看读者实际看到的样子 —— 此前这个按钮
 * 只挂了 Markdown 渲染，docx 档位下点开是空白。
 */
import PublishMarkdown from './PublishMarkdown.vue'
import DocxDocument from './DocxDocument.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import type { PublishSubset } from '@/utils/publishMarkdown'

const visible = defineModel<boolean>({ default: false })

const props = withDefaults(
  defineProps<{
    content?: string
    /** chapter：可信面全集（表格 / 高亮 / 公式）；featured：三端交集（表格与公式降级为文本）。 */
    subset?: PublishSubset
    title?: string
    /** 副标题：说明这份预览是「给谁看的」，属必要的功能性提示（不是装饰性 hint）。 */
    subtitle?: string
    /**
     * 原始 .docx 的地址（内容精选 `content_type = docx`）。
     * 给了就改渲染 Word，不再看 `content` —— 两种形态互斥，由后端保证同时只有一列有值。
     */
    docxUrl?: string
  }>(),
  { content: '', subset: 'chapter', title: '发布端预览', subtitle: '', docxUrl: '' }
)
</script>

<template>
  <UiDialog v-model="visible" :title="props.title" :subtitle="props.subtitle" width="800px" destroy-on-close>
    <DocxDocument
      v-if="props.docxUrl"
      class="max-h-[60vh] overflow-y-auto rounded-ctl border border-line bg-canvas p-4"
      :src="props.docxUrl"
    />
    <PublishMarkdown
      v-else
      class="max-h-[60vh] overflow-y-auto rounded-ctl border border-line bg-canvas p-4"
      :subset="props.subset"
      :content="props.content"
    />
    <template #footer>
      <UiButton @click="visible = false">关闭</UiButton>
    </template>
  </UiDialog>
</template>
