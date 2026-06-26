<template>
  <div class="image-viewer" ref="containerRef">
    <div class="image-toolbar">
      <el-tooltip content="缩小" placement="bottom">
        <el-button :icon="ZoomOut" circle size="small" @click="zoomOut" :disabled="scale <= 0.1" />
      </el-tooltip>
      <span class="zoom-text">{{ Math.round(scale * 100) }}%</span>
      <el-tooltip content="放大" placement="bottom">
        <el-button :icon="ZoomIn" circle size="small" @click="zoomIn" :disabled="scale >= 5" />
      </el-tooltip>
      <el-divider direction="vertical" />
      <el-tooltip content="适合窗口" placement="bottom">
        <el-button :icon="FullScreen" circle size="small" @click="fitToWindow" />
      </el-tooltip>
      <el-tooltip content="原始大小" placement="bottom">
        <el-button :icon="RefreshRight" circle size="small" @click="resetZoom" />
      </el-tooltip>
      <el-divider direction="vertical" />
      <el-tooltip content="全屏" placement="bottom">
        <el-button :icon="Rank" circle size="small" @click="toggleFullscreen" />
      </el-tooltip>
      <el-tooltip content="下载" placement="bottom">
        <el-button :icon="Download" circle size="small" @click="downloadFile" />
      </el-tooltip>
    </div>

    <div
      class="image-canvas"
      ref="canvasRef"
      @wheel.prevent="onWheel"
      @mousedown="onMouseDown"
      @mousemove="onMouseMove"
      @mouseup="onMouseUp"
      @mouseleave="onMouseUp"
      @touchstart.prevent="onTouchStart"
      @touchmove.prevent="onTouchMove"
      @touchend="onTouchEnd"
    >
      <img
        ref="imgRef"
        :src="resolvedSrc"
        :style="imageStyle"
        class="viewer-image"
        @load="onImageLoad"
        @error="onImageError"
        draggable="false"
      />
      <div v-if="imgError" class="image-error">
        <el-empty :description="imgErrorMessage">
          <el-button type="primary" @click="retryLoad">重试</el-button>
        </el-empty>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ZoomIn, ZoomOut, FullScreen, Download, RefreshRight, Rank } from '@element-plus/icons-vue'
import { resolveFileUrl } from '@/utils/fileUrl'

const props = defineProps({
  src: { type: String, required: true },
  fileName: { type: String, default: '' }
})

const resolvedSrc = computed(() => resolveFileUrl(props.src))

const containerRef = ref(null)
const canvasRef = ref(null)
const imgRef = ref(null)

const scale = ref(1)
const panX = ref(0)
const panY = ref(0)
const isDragging = ref(false)
const dragStartX = ref(0)
const dragStartY = ref(0)
const panStartX = ref(0)
const panStartY = ref(0)
const imgError = ref(false)
const imgErrorMessage = ref('图片加载失败')
const imgLoaded = ref(false)
const naturalWidth = ref(0)
const naturalHeight = ref(0)

const initialPinchDistance = ref(0)
const initialPinchScale = ref(1)

const imageStyle = computed(() => ({
  transform: `translate(${panX.value}px, ${panY.value}px) scale(${scale.value})`,
  cursor: isDragging.value ? 'grabbing' : scale.value > 1 ? 'grab' : 'default',
  transition: isDragging.value ? 'none' : 'transform 0.2s ease',
  transformOrigin: 'center center'
}))

function onImageLoad() {
  imgLoaded.value = true
  imgError.value = false
  naturalWidth.value = imgRef.value.naturalWidth
  naturalHeight.value = imgRef.value.naturalHeight
  fitToWindow()
}

function onImageError() {
  imgError.value = true
  imgLoaded.value = false
  checkFileExists()
}

async function checkFileExists() {
  try {
    const response = await fetch(resolvedSrc.value, { method: 'HEAD' })
    if (response.status === 404) {
      imgErrorMessage.value = '图片文件不存在或已过期，请重新上传'
    } else if (!response.ok) {
      imgErrorMessage.value = `图片加载失败 (${response.status})`
    }
  } catch (e) {
    imgErrorMessage.value = '无法连接到文件服务器'
  }
}

function retryLoad() {
  imgError.value = false
  if (imgRef.value) {
    imgRef.value.src = ''
    setTimeout(() => { imgRef.value.src = props.src }, 100)
  }
}

function zoomIn() {
  const newScale = Math.min(scale.value + 0.2, 5)
  scale.value = Math.round(newScale * 10) / 10
}

function zoomOut() {
  const newScale = Math.max(scale.value - 0.2, 0.1)
  scale.value = Math.round(newScale * 10) / 10
}

function fitToWindow() {
  if (!canvasRef.value || !naturalWidth.value) return
  const cw = canvasRef.value.clientWidth
  const ch = canvasRef.value.clientHeight
  const padding = 40
  const scaleX = (cw - padding) / naturalWidth.value
  const scaleY = (ch - padding) / naturalHeight.value
  scale.value = Math.round(Math.min(scaleX, scaleY, 1) * 100) / 100
  panX.value = 0
  panY.value = 0
}

function resetZoom() {
  scale.value = 1
  panX.value = 0
  panY.value = 0
}

function onWheel(e) {
  const delta = e.ctrlKey ? 0.02 : 0.1
  if (e.deltaY < 0) {
    scale.value = Math.min(Math.round((scale.value + delta) * 100) / 100, 5)
  } else {
    scale.value = Math.max(Math.round((scale.value - delta) * 100) / 100, 0.1)
  }
}

function onMouseDown(e) {
  if (e.button !== 0) return
  if (scale.value <= 1) return
  isDragging.value = true
  dragStartX.value = e.clientX
  dragStartY.value = e.clientY
  panStartX.value = panX.value
  panStartY.value = panY.value
}

function onMouseMove(e) {
  if (!isDragging.value) return
  panX.value = panStartX.value + (e.clientX - dragStartX.value)
  panY.value = panStartY.value + (e.clientY - dragStartY.value)
}

function onMouseUp() {
  isDragging.value = false
}

function onTouchStart(e) {
  if (e.touches.length === 2) {
    const dx = e.touches[0].clientX - e.touches[1].clientX
    const dy = e.touches[0].clientY - e.touches[1].clientY
    initialPinchDistance.value = Math.sqrt(dx * dx + dy * dy)
    initialPinchScale.value = scale.value
  } else if (e.touches.length === 1) {
    isDragging.value = true
    dragStartX.value = e.touches[0].clientX
    dragStartY.value = e.touches[0].clientY
    panStartX.value = panX.value
    panStartY.value = panY.value
  }
}

function onTouchMove(e) {
  if (e.touches.length === 2) {
    const dx = e.touches[0].clientX - e.touches[1].clientX
    const dy = e.touches[0].clientY - e.touches[1].clientY
    const distance = Math.sqrt(dx * dx + dy * dy)
    const newScale = initialPinchScale.value * (distance / initialPinchDistance.value)
    scale.value = Math.round(Math.min(Math.max(newScale, 0.1), 5) * 100) / 100
  } else if (e.touches.length === 1 && isDragging.value) {
    panX.value = panStartX.value + (e.touches[0].clientX - dragStartX.value)
    panY.value = panStartY.value + (e.touches[0].clientY - dragStartY.value)
  }
}

function onTouchEnd() {
  isDragging.value = false
}

function toggleFullscreen() {
  if (!containerRef.value) return
  if (document.fullscreenElement) {
    document.exitFullscreen()
  } else {
    containerRef.value.requestFullscreen()
  }
}

function downloadFile() {
  const link = document.createElement('a')
  link.href = resolvedSrc.value
  link.download = props.fileName || ''
  link.click()
}

function onFullscreenChange() {
  if (!document.fullscreenElement && imgLoaded.value) {
    fitToWindow()
  }
}

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
})

onBeforeUnmount(() => {
  document.removeEventListener('fullscreenchange', onFullscreenChange)
})
</script>

<style scoped>
.image-viewer {
  width: 100%;
  height: 100%;
  min-height: 500px;
  display: flex;
  flex-direction: column;
  background: #1a1a2e;
  border-radius: 8px;
  overflow: hidden;
}

.image-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 12px;
  background: #16213e;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  flex-shrink: 0;
}

.image-toolbar .el-button {
  --el-button-bg-color: transparent;
  --el-button-border-color: transparent;
  --el-button-text-color: #ccc;
  --el-button-hover-bg-color: rgba(255, 255, 255, 0.1);
  --el-button-hover-border-color: transparent;
  --el-button-hover-text-color: #fff;
}

.zoom-text {
  color: #ccc;
  font-size: 13px;
  min-width: 48px;
  text-align: center;
  user-select: none;
}

.image-canvas {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  position: relative;
}

.viewer-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  user-select: none;
  -webkit-user-drag: none;
}

.image-error {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
}

@media screen and (max-width: 767px) {
  .image-toolbar {
    flex-wrap: wrap;
    padding: 6px 8px;
    gap: 4px;
  }

  .image-toolbar .el-divider {
    display: none;
  }

  .zoom-text {
    min-width: 36px;
    font-size: 12px;
  }
}
</style>
