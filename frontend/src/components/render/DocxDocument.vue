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
/** 渲染完发现加载不出来的图片数（docx-preview 支持不了的格式 / 外部链接） */
const brokenImages = ref(0)
/** 竞态守卫：换文件/卸载时作废在飞的渲染，避免旧文件后到覆盖新内容 */
let renderToken = 0

function clearContainers() {
  if (bodyRef.value) bodyRef.value.innerHTML = ''
  if (styleRef.value) styleRef.value.innerHTML = ''
  brokenImages.value = 0
}

/**
 * 把 docx-preview 渲染不出来的图片标出来（用户反馈「图片有时候加载不出来」）。
 *
 * 实测两类它支持不了、且**静默留白**的情形（都是真实文档里常见的）：
 *   1. **外部链接图片**（Word 的「链接到文件」，`TargetMode="External"`）：
 *      `loadDocumentImage` 只按包内关系找字节，找不到就把 `img.src` 设成 `null`
 *      —— 实测 `getAttribute('src') === 'null'`、`naturalWidth === 0`。
 *   2. **浏览器不认的矢量格式**（EMF / WMF，Windows 版 Word 粘贴图片的默认格式）：
 *      字节取到了、blob URL 也生成了，但浏览器解不了 ⇒ `complete === true` 而
 *      `naturalWidth === 0`，页面上就是一块按原始尺寸撑开的空白。
 *
 * 这两种都不该由我们悄悄吞掉：留白的图片比明确的失败提示更难排查。
 * 这里不改 DOM 结构（docx-preview 的排版尺寸要保住，否则整页会塌），只给每张
 * 失败的图加一个 `data-docx-image-failed` 标记，由 CSS 画出可见的失败态。
 */
function markBrokenImages(root: HTMLElement) {
  const images = Array.from(root.querySelectorAll('img'))
  let broken = 0
  for (const img of images) {
    // `src="null"` 是 docx-preview 在找不到包内图片时的固定产物（字符串 "null"，不是 null）
    const src = img.getAttribute('src')
    const missingSrc = !src || src === 'null'
    // complete 且 naturalWidth 为 0 ⇒ 字节在、但浏览器解码不了（EMF/WMF 等）
    const undecodable = img.complete && img.naturalWidth === 0
    if (missingSrc || undecodable) {
      broken += 1
      img.setAttribute('data-docx-image-failed', missingSrc ? 'external' : 'format')
      img.setAttribute('alt', missingSrc ? '（外部链接的图片，无法显示）' : '（该图片格式无法在浏览器中显示）')
    }
  }
  brokenImages.value = broken
}

/**
 * 等这一批图片加载完再复查一次失败态。
 *
 * 必要性：`renderAsync` 返回时 `img.src` 刚被赋值，浏览器还没解码 —— 此刻
 * `naturalWidth` 一律是 0，直接判会把**正常图片**也标成失败（实测第一版就这样误伤）。
 * 等到 load / error 事件落地后再判，才分得清「还在加载」与「真的解不开」。
 */
async function waitForImages(root: HTMLElement, token: number) {
  const pending = Array.from(root.querySelectorAll('img')).filter((img) => !img.complete)
  if (pending.length > 0) {
    await Promise.all(
      pending.map(
        (img) =>
          new Promise<void>((resolve) => {
            const done = () => {
              img.removeEventListener('load', done)
              img.removeEventListener('error', done)
              resolve()
            }
            img.addEventListener('load', done, { once: true })
            img.addEventListener('error', done, { once: true })
          })
      )
    )
  }
  if (token === renderToken) markBrokenImages(root)
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
    // 图片是 renderAsync 内部按 task 异步塞 src 的，走到这里 src 已落定；
    // 但「字节到了、解码失败」要等浏览器解码完才有 naturalWidth，故再等一轮图片事件
    markBrokenImages(bodyRef.value)
    void waitForImages(bodyRef.value, token)
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
    <!--
      部分图片显示不出来时的提示（2026-11 反馈「图片有时候加载不出来」）。
      不做成整块失败态：正文本身是好的，只有个别图片解不开 —— 按图片数说清楚是哪一类，
      并把「怎么改」写出来，否则作者只会以为是我们坏了。
    -->
    <UiAlert
      v-if="brokenImages > 0"
      class="mb-2"
      type="warning"
      :closable="false"
      show-icon
      :title="`有 ${brokenImages} 张图片无法显示`"
      description="文档里的这些图片要么是链接到外部文件（Word 的「链接到文件」），要么是浏览器不支持的矢量格式（EMF/WMF）。请在 Word 里把图片改为「嵌入」，或另存为 PNG/JPG 后重新上传。"
    />
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

/*
 * 加载不出来的图片画成可见的占位块。
 *
 * 为什么保留原尺寸而不 `display:none`：docx-preview 是按 OOXML 的绝对尺寸排版的，
 * 抽掉图片会让整页版心塌陷、文字位置全变（比留白更难对上原文）。所以只把这块
 * 空白「画出来」——斜纹底 + 虚线边框，一眼能看出这里本该有图。
 *
 * 用 `outline` 之外的纯视觉属性（背景 / 边框）而不是伪元素：img 是替换元素，
 * ::before / ::after 不参与渲染。
 */
.docx-document__body :deep(img[data-docx-image-failed]) {
  display: block;
  min-width: 96px;
  min-height: 72px;
  box-sizing: border-box;
  background-color: var(--color-bg-page);
  background-image: repeating-linear-gradient(
    45deg,
    transparent,
    transparent 6px,
    var(--color-line) 6px,
    var(--color-line) 7px
  );
  border: 1px dashed var(--color-line-strong);
  border-radius: var(--radius-sm);
}
</style>
