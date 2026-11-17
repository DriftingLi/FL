<script setup lang="ts">
/**
 * docx 正文渲染（2026-11）：内容精选 `content_type = docx` 的读者面单点。
 *
 * 为什么是客户端渲染而不是服务端转 HTML/PDF：用户要的是「直接渲染 .docx」，
 * 转换会丢页眉页脚、多级列表缩进、合并单元格这些正是 Word 里排版的部分；
 * docx-preview 按 OOXML 在浏览器里还原，作者看到的就是 Word 里的样子。
 *
 * ⚠️ 三件必须一起做的事（少一件就静默出空白）：
 *   1. 用 `renderAsync` 前先 `fetch` 成 Blob —— 直接传 URL 不支持跨域；
 *   2. 样式表挂到**第二个容器**（docx-preview 会往里塞一段 <style>），
 *      挂到 body 容器会与内容混在一起被再次清空；
 *   3. 组件卸载/换文件时先清空两个容器，否则重渲染会在旧节点后追加一份。
 *
 * 移动端（uni-app x）没有 DOM，走不到这里：那边由解析器显示摘要 + 提示语（ADR-0046 已知限制）。
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { renderAsync } from 'docx-preview'
import UiAlert from '@/components/ui/UiAlert.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const props = withDefaults(
  defineProps<{
    /** .docx 文件 URL（后端 featured/docx/ 子目录） */
    src?: string
  }>(),
  { src: '' }
)

const bodyRef = ref<HTMLElement | null>(null)
const styleRef = ref<HTMLElement | null>(null)
const loading = ref(false)
const failed = ref(false)
/** 竞态守卫：换文件/卸载时作废在飞的渲染，避免旧文件后到覆盖新内容 */
let renderToken = 0

function clearContainers() {
  if (bodyRef.value) bodyRef.value.innerHTML = ''
  if (styleRef.value) styleRef.value.innerHTML = ''
}

async function renderDocx(url: string) {
  const token = ++renderToken
  clearContainers()
  failed.value = false
  if (!url || !bodyRef.value) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const res = await fetch(url)
    if (!res.ok) throw new Error(`下载文档失败：${res.status}`)
    const blob = await res.blob()
    if (token !== renderToken) return // 期间换了文件
    // breakPages:false ⇒ 连续流式排版更像网页正文；renderHeaders/Footers 保留页眉页脚
    await renderAsync(blob, bodyRef.value, styleRef.value ?? undefined, {
      inWrapper: true,
      breakPages: false,
      ignoreWidth: false,
      renderHeaders: true,
      renderFooters: true,
      useBase64URL: false
    })
    if (token !== renderToken) {
      clearContainers()
      return
    }
  } catch (e) {
    if (token === renderToken) failed.value = true
  } finally {
    if (token === renderToken) loading.value = false
  }
}

// 首屏必须在 onMounted 后才渲染：immediate watch 在挂载前跑，那时 bodyRef 还是 null，
// 会因为「没有容器」静默返回、整块正文空白（这是被 DocxDocument.spec 抓到的真实缺陷）。
onMounted(() => {
  void renderDocx(props.src)
})

watch(
  () => props.src,
  (url) => {
    void renderDocx(url)
  }
)

onBeforeUnmount(() => {
  renderToken++ // 作废在飞的渲染，避免它往已卸载的容器里写
  clearContainers()
})
</script>

<template>
  <div class="docx-document">
    <UiAlert
      v-if="failed"
      type="warning"
      :closable="false"
      show-icon
      title="文档加载失败"
      description="文件可能已被移除，或网络异常。请刷新重试，或联系管理员。"
    />
    <UiSkeleton v-else-if="loading" variant="card" :count="2" />
    <!-- docx-preview 的样式容器：它会把一段 <style> 塞进来，必须与正文分开 -->
    <div ref="styleRef" class="docx-document__style" />
    <div ref="bodyRef" class="docx-document__body" />
  </div>
</template>

<style scoped>
.docx-document {
  /* docx-preview 输出的是固定版心宽度的 .docx-wrapper：窄屏靠横向滚动兜住，不压扁排版 */
  overflow-x: auto;
}

.docx-document__style {
  display: none;
}

.docx-document__body :deep(.docx-wrapper) {
  background: transparent;
  padding: 0;
}

.docx-document__body :deep(.docx-wrapper > section.docx) {
  margin-bottom: var(--space-4, 16px);
  box-shadow: var(--shadow-xs);
  border-radius: var(--radius-md);
}

.docx-document__body :deep(table) {
  max-width: 100%;
}
</style>
