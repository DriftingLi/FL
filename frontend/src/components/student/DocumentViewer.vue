<template>
  <div class="document-viewer">
    <div class="doc-toolbar">
      <div class="toolbar-left">
        <el-icon :size="18" class="toolbar-icon"><Document /></el-icon>
        <span class="file-name" :title="fileName">{{ fileName || '文档' }}</span>
      </div>
      <div class="toolbar-right">
        <el-tooltip v-if="canPreview" content="在新窗口打开" placement="bottom">
          <UiButton :icon="FullScreen" circle size="small" @click="openInNewTab"/>
        </el-tooltip>
        <el-tooltip content="下载" placement="bottom">
          <UiButton :icon="Download" circle size="small" @click="downloadFile"/>
        </el-tooltip>
      </div>
    </div>

    <div class="doc-body">
      <!-- PDF 用 iframe 内嵌预览 -->
      <iframe
        v-if="canPreview && !loadError && resolvedSrc"
        :src="resolvedSrc"
        class="pdf-iframe"
        frameborder="0"
        @load="onIframeLoad"
        @error="onIframeError"
      ></iframe>

      <div v-if="loading && canPreview" class="doc-loading">
        <el-icon class="loading-icon" :size="32"><Loading /></el-icon>
        <span>文档加载中...</span>
      </div>

      <!-- 非 PDF 文档：浏览器无法内嵌预览，提供下载/新窗口打开 -->
      <div v-if="!canPreview" class="doc-unsupported">
        <el-empty :description="unsupportedMessage">
          <UiButton variant="primary" @click="downloadFile">
            <el-icon><Download /></el-icon> 下载文档
          </UiButton>
          <UiButton @click="openInNewTab">在新窗口打开</UiButton>
        </el-empty>
        <p class="unsupported-tip" v-if="unsupportedTip">{{ unsupportedTip }}</p>
      </div>

      <div v-if="loadError" class="doc-error">
        <el-empty :description="errorMessage">
          <UiButton variant="primary" @click="downloadFile">下载文档</UiButton>
          <UiButton @click="openInNewTab">在新窗口打开</UiButton>
        </el-empty>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { Document, Download, FullScreen, Loading } from '@element-plus/icons-vue'
import { resolveFileUrl } from '@/utils/fileUrl'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps({
  src: { type: String, required: true },
  fileName: { type: String, default: '' }
})

const resolvedSrc = computed(() => resolveFileUrl(props.src))

const loading = ref(true)
const loadError = ref(false)
const errorMessage = ref('文档加载失败')

// 文件扩展名（小写）
const fileExt = computed(() => {
  const name = props.fileName || props.src || ''
  const idx = name.lastIndexOf('.')
  if (idx < 0) return ''
  return name.slice(idx + 1).toLowerCase()
})

// 是否可在浏览器内嵌预览（PDF 可由浏览器原生渲染）
const canPreview = computed(() => fileExt.value === 'pdf')

// Office 文档类型描述
const officeTypeLabel = computed(() => {
  const ext = fileExt.value
  if (['doc', 'docx'].includes(ext)) return 'Word 文档'
  if (['xls', 'xlsx'].includes(ext)) return 'Excel 表格'
  if (['ppt', 'pptx'].includes(ext)) return 'PPT 演示文稿'
  if (ext === 'csv') return 'CSV 文件'
  return '文档'
})

const unsupportedMessage = computed(() => `${officeTypeLabel.value}无法在浏览器中直接预览，请下载后查看`)

const unsupportedTip = computed(() => {
  const ext = fileExt.value
  if (['doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx'].includes(ext)) {
    return '提示：PPT/Word/Excel 文件请下载后使用对应软件打开'
  }
  return ''
})

async function checkFileExists() {
  try {
    const response = await fetch(resolvedSrc.value, { method: 'HEAD' })
    if (!response.ok) {
      loadError.value = true
      if (response.status === 404) {
        errorMessage.value = '文件不存在或已过期，请重新上传'
      } else {
        errorMessage.value = `文档加载失败 (${response.status})`
      }
    }
  } catch (e) {
    loadError.value = true
    errorMessage.value = '无法连接到文件服务器'
  } finally {
    loading.value = false
  }
}

function onIframeLoad() {
  loading.value = false
}

function onIframeError() {
  loadError.value = true
  errorMessage.value = '文档加载失败'
  loading.value = false
}

function openInNewTab() {
  window.open(resolvedSrc.value, '_blank')
}

function downloadFile() {
  const link = document.createElement('a')
  link.href = resolvedSrc.value
  link.download = props.fileName || 'document'
  link.target = '_blank'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

onMounted(() => {
  // 仅 PDF 需要检查文件存在性并加载 iframe
  if (canPreview.value) {
    checkFileExists()
  } else {
    loading.value = false
  }
})

watch(resolvedSrc, (newVal) => {
  if (newVal) {
    loading.value = true
    loadError.value = false
    if (canPreview.value) {
      checkFileExists()
    } else {
      loading.value = false
    }
  }
})
</script>

<style scoped>
.document-viewer {
  width: 100%;
  height: 600px;
  display: flex;
  flex-direction: column;
  background: var(--color-viewer-bg);
  border-radius: 8px;
  overflow: hidden;
  position: relative;
}

.doc-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--color-viewer-bar);
  border-bottom: 1px solid var(--color-viewer-line);
  flex-shrink: 0;
  gap: 8px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.toolbar-icon {
  color: var(--color-primary-500);
  flex-shrink: 0;
}

.file-name {
  color: var(--color-viewer-text);
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.doc-toolbar .el-button {
  --el-button-bg-color: transparent;
  --el-button-border-color: transparent;
  --el-button-text-color: var(--color-viewer-text);
  --el-button-hover-bg-color: rgba(255, 255, 255, 0.1);
  --el-button-hover-border-color: transparent;
  --el-button-hover-text-color: #fff;
}

.doc-body {
  flex: 1;
  position: relative;
  overflow: hidden;
}

.pdf-iframe {
  width: 100%;
  height: 100%;
  border: none;
  display: block;
}

.doc-loading {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: rgba(82, 86, 89, 0.9);
  color: var(--color-viewer-text);
  z-index: 10;
}

.doc-unsupported,
.doc-error {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(82, 86, 89, 0.95);
  z-index: 10;
}

.unsupported-tip {
  margin-top: 12px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  text-align: center;
  padding: 0 20px;
}

.loading-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media screen and (max-width: 767px) {
  .document-viewer {
    height: 500px;
  }

  .file-name {
    font-size: 13px;
  }
}
</style>
