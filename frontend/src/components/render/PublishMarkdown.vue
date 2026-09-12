<script setup lang="ts">
/**
 * 发布端渲染组件（ADR-0046 / #903）。
 *
 * 学员阅读页与讲师/管理员编辑页预览共用这一个组件、共用 `renderPublishMarkdown` 一个实现。
 * 组件很薄，价值全在「只有一个」：任何绕过它自己写 `v-html` 的地方都会重新制造
 * 「编辑端所见 ≠ 发布端所得」。
 *
 * ⚠️ 模板**必须只有一个根节点**：调用方靠 attrs 透传排版类（字号/行高/颜色），
 * 变成 fragment 会让这些类静默失效（ForumContent 已踩过同一个坑）。
 */
import { computed } from 'vue'
import '@/assets/styles/markdown.css'
import {
  detectOutsideSubset,
  outsideSubsetNotice,
  renderPublishMarkdown,
  type PublishSubset
} from '@/utils/publishMarkdown'
import UiAlert from '@/components/ui/UiAlert.vue'

const props = withDefaults(
  defineProps<{
    content?: string
    /** chapter：可信面全集（表格+高亮+公式）；featured：三端交集（表格/公式降级为文本）。 */
    subset?: PublishSubset
  }>(),
  { content: '', subset: 'chapter' }
)

const html = computed(() => renderPublishMarkdown(props.content, props.subset))

/**
 * 交集档的越界说明：只在**真的越界**时出现，且不阻断任何操作。
 * 放在组件里而不是页面里，是为了「哪个渲染档位就要给哪个档位的说明」不依赖调用方记得加。
 */
const outsideLabels = computed(() => (props.subset === 'featured' ? detectOutsideSubset(props.content) : []))

const outsideNotice = computed(() => outsideSubsetNotice(outsideLabels.value))
</script>

<template>
  <div class="publish-markdown">
    <UiAlert
      v-if="outsideLabels.length"
      class="mb-3"
      type="warning"
      :closable="false"
      show-icon
      :title="outsideNotice"
    />
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div class="markdown-body" v-html="html"></div>
  </div>
</template>
