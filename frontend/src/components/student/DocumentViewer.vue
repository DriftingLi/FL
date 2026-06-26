<template>
  <div class="document-viewer">
    <div class="doc-toolbar">
      <div class="toolbar-left">
        <el-icon :size="18" class="toolbar-icon"><Document /></el-icon>
        <span class="file-name" :title="fileName">{{ fileName || 'PDF文档' }}</span>
      </div>
      <div class="toolbar-right">
        <el-tooltip content="在新窗口打开" placement="bottom">
          <el-button :icon="FullScreen" circle size="small" @click="openInNewTab" />
        </el-tooltip>
        <el-tooltip content="下载" placement="bottom">
          <el-button :icon="Download" circle size="small" @click="downloadFile" />
        </el-tooltip>
      </div>
    </div>

    <div class="doc-body">
      <iframe
        v-if="!loadError && resolvedSrc"
        :src="resolvedSrc"
        class="pdf-iframe"
        frameborder="0"
        @load="onIframeLoad"
        @error="onIframeError"
      ></iframe>

      <div v-if="loading" class="doc-loading">
        <el-icon class="loading-icon" :size="32"><Loading /></el-icon>
        <span>文档加载中...</span>
      </div>

      <div v-if="loadError" class="doc-error">
        <el-empty :description="errorMessage">
          <el-button type="primary" @click="downloadFile">下载文档</el-button>
          <el-button @click="openInNewTab">在新窗口打开</el-button>
        </el-empty>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { Document, Download, FullScreen, Loading } from '@element-plus/icons-vue'
import { resolveFileUrl } from '@/utils/fileUrl'

const props = defineProps({
  src: { type: String, required: true },
  fileName: { type: String, default: '' }
})

const resolvedSrc = computed(() => resolveFileUrl(props.src))

const loading = ref(true)
const loadError = ref(false)
const errorMessage = ref('文档加载失败')

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
  link.download = props.fileName || 'document.pdf'
  link.target = '_blank'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

onMounted(() => {
  checkFileExists()
})

watch(resolvedSrc, (newVal) => {
  if (newVal) {
    loading.value = true
    loadError.value = false
    checkFileExists()
  }
})
</script>

<style scoped>
.document-viewer {
  width: 100%;
  height: 600px;
  display: flex;
  flex-direction: column;
  background: #525659;
  border-radius: 8px;
  overflow: hidden;
  position: relative;
}

.doc-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #3c3f41;
  border-bottom: 1px solid #555;
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
  color: #409eff;
  flex-shrink: 0;
}

.file-name {
  color: #ddd;
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
  --el-button-text-color: #ccc;
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
  color: #ccc;
  z-index: 10;
}

.loading-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.doc-error {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(82, 86, 89, 0.95);
  z-index: 10;
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
