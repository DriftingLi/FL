<template>
  <div class="ppt-viewer" ref="containerRef">
    <div v-if="loading" class="ppt-loading">
      <el-icon class="loading-icon" :size="32"><Loading /></el-icon>
      <span>幻灯片加载中...</span>
    </div>

    <div v-else-if="loadError" class="ppt-error">
      <el-empty description="幻灯片加载失败">
        <el-button type="primary" @click="loadSlides">重试</el-button>
        <el-button @click="downloadFile">下载PPT</el-button>
      </el-empty>
    </div>

    <div v-else-if="slides.length === 0" class="ppt-empty">
      <el-empty description="暂无幻灯片预览">
        <el-button type="primary" @click="downloadFile">下载PPT文件</el-button>
      </el-empty>
    </div>

    <template v-else>
      <div class="ppt-header">
        <span class="slide-title">{{ fileName }}</span>
        <div class="ppt-actions">
          <el-tooltip content="重新生成幻灯片" placement="bottom">
            <el-button :icon="Refresh" circle size="small" :loading="regenerating" @click="regenerateSlides" />
          </el-tooltip>
          <el-tooltip content="全屏演示" placement="bottom">
            <el-button :icon="Rank" circle size="small" @click="toggleFullscreen" />
          </el-tooltip>
          <el-tooltip content="下载" placement="bottom">
            <el-button :icon="Download" circle size="small" @click="downloadFile" />
          </el-tooltip>
        </div>
      </div>

      <div class="ppt-main">
        <div
          class="slide-display"
          @touchstart="onSlideTouchStart"
          @touchmove="onSlideTouchMove"
          @touchend="onSlideTouchEnd"
        >
          <transition name="slide-fade" mode="out-in">
            <img
              :key="currentSlideIndex"
              :src="slides[currentSlideIndex]"
              class="slide-image"
              @click="toggleFullscreen"
              @error="onSlideImageError"
            />
          </transition>
        </div>
      </div>

      <div class="ppt-footer">
        <div class="slide-nav">
          <el-button :icon="ArrowLeft" circle @click="prevSlide" :disabled="currentSlideIndex <= 0" />
          <span class="slide-counter">{{ currentSlideIndex + 1 }} / {{ slides.length }}</span>
          <el-button :icon="ArrowRight" circle @click="nextSlide" :disabled="currentSlideIndex >= slides.length - 1" />
        </div>
      </div>

      <div class="thumbnail-strip" ref="stripRef">
        <div class="thumbnail-scroll">
          <div
            v-for="(slide, index) in slides"
            :key="index"
            class="thumbnail-item"
            :class="{ active: index === currentSlideIndex }"
            @click="goToSlide(index)"
          >
            <img :src="slide" class="thumbnail-image" />
            <span class="thumbnail-num">{{ index + 1 }}</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { ArrowLeft, ArrowRight, Download, Rank, Loading, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { courseApi } from '@/api/course'
import { resolveFileUrl } from '@/utils/fileUrl'

const props = defineProps({
  src: { type: String, required: true },
  fileName: { type: String, default: '' },
  chapterId: { type: [Number, String], required: true }
})

const resolvedSrc = computed(() => resolveFileUrl(props.src))

const containerRef = ref(null)
const stripRef = ref(null)

const slides = ref([])
const currentSlideIndex = ref(0)
const loading = ref(true)
const loadError = ref(false)
const regenerating = ref(false)

const touchStartX = ref(0)
const touchStartY = ref(0)
const SWIPE_THRESHOLD = 50

async function loadSlides() {
  loading.value = true
  loadError.value = false
  try {
    const res = await courseApi.getChapterSlides(props.chapterId)
    let rawSlides = []
    if (res && res.data && Array.isArray(res.data.slides)) {
      rawSlides = res.data.slides
    } else if (res && Array.isArray(res.slides)) {
      rawSlides = res.slides
    } else if (res && Array.isArray(res)) {
      rawSlides = res
    }
    slides.value = rawSlides.map(s => resolveFileUrl(s))
    currentSlideIndex.value = 0
    if (slides.value.length === 0) {
      loadError.value = true
    }
  } catch (e) {
    console.error('Failed to load slides:', e)
    loadError.value = true
  } finally {
    loading.value = false
  }
}

function prevSlide() {
  if (currentSlideIndex.value > 0) {
    currentSlideIndex.value--
    scrollToThumbnail()
  }
}

function nextSlide() {
  if (currentSlideIndex.value < slides.value.length - 1) {
    currentSlideIndex.value++
    scrollToThumbnail()
  }
}

function goToSlide(index) {
  currentSlideIndex.value = index
  scrollToThumbnail()
}

function scrollToThumbnail() {
  if (!stripRef.value) return
  const activeThumb = stripRef.value.querySelector('.thumbnail-item.active')
  if (activeThumb) {
    activeThumb.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' })
  }
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
  link.target = '_blank'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

function onSlideImageError() {
  loadError.value = true
}

async function regenerateSlides() {
  regenerating.value = true
  try {
    await courseApi.regenerateSlides(props.chapterId)
    ElMessage.success('幻灯片重新生成成功')
    await loadSlides()
  } catch (e) {
    console.error('Failed to regenerate slides:', e)
    ElMessage.error('重新生成幻灯片失败')
  } finally {
    regenerating.value = false
  }
}

function onSlideTouchStart(e) {
  if (e.touches.length === 1) {
    touchStartX.value = e.touches[0].clientX
    touchStartY.value = e.touches[0].clientY
  }
}

function onSlideTouchMove(e) {
  e.preventDefault()
}

function onSlideTouchEnd(e) {
  if (e.changedTouches.length !== 1) return
  const dx = e.changedTouches[0].clientX - touchStartX.value
  const dy = e.changedTouches[0].clientY - touchStartY.value
  if (Math.abs(dx) < SWIPE_THRESHOLD) return
  if (Math.abs(dx) < Math.abs(dy)) return
  if (dx < 0) {
    nextSlide()
  } else {
    prevSlide()
  }
}

function onKeyDown(e) {
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return
  switch (e.key) {
    case 'ArrowLeft':
      e.preventDefault()
      prevSlide()
      break
    case 'ArrowRight':
      e.preventDefault()
      nextSlide()
      break
    case 'f':
    case 'F':
      e.preventDefault()
      toggleFullscreen()
      break
    case 'Escape':
      if (document.fullscreenElement) {
        document.exitFullscreen()
      }
      break
  }
}

onMounted(() => {
  loadSlides()
  document.addEventListener('keydown', onKeyDown)
})

watch(() => props.chapterId, (newVal) => {
  if (newVal) {
    slides.value = []
    currentSlideIndex.value = 0
    loadSlides()
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKeyDown)
})
</script>

<style scoped>
.ppt-viewer {
  width: 100%;
  height: 600px;
  display: flex;
  flex-direction: column;
  background: #1a1a2e;
  border-radius: 8px;
  overflow: hidden;
  position: relative;
}

.ppt-viewer:fullscreen {
  background: #0d0d1a;
}

.ppt-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: #16213e;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  flex-shrink: 0;
}

.slide-title {
  color: #ccc;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 60%;
}

.ppt-actions {
  display: flex;
  gap: 4px;
}

.ppt-actions .el-button {
  --el-button-bg-color: transparent;
  --el-button-border-color: transparent;
  --el-button-text-color: #ccc;
  --el-button-hover-bg-color: rgba(255, 255, 255, 0.1);
  --el-button-hover-border-color: transparent;
  --el-button-hover-text-color: #fff;
}

.ppt-main {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  overflow: hidden;
}

.slide-display {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.slide-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 4px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
  cursor: pointer;
}

.slide-fade-enter-active,
.slide-fade-leave-active {
  transition: opacity 0.25s ease;
}

.slide-fade-enter-from,
.slide-fade-leave-to {
  opacity: 0;
}

.ppt-footer {
  display: flex;
  justify-content: center;
  padding: 8px 16px;
  flex-shrink: 0;
}

.slide-nav {
  display: flex;
  align-items: center;
  gap: 12px;
}

.slide-nav .el-button {
  --el-button-bg-color: rgba(255, 255, 255, 0.1);
  --el-button-border-color: rgba(255, 255, 255, 0.2);
  --el-button-text-color: #ccc;
  --el-button-hover-bg-color: rgba(255, 255, 255, 0.2);
  --el-button-hover-border-color: rgba(255, 255, 255, 0.3);
  --el-button-hover-text-color: #fff;
}

.slide-counter {
  color: #ccc;
  font-size: 14px;
  min-width: 60px;
  text-align: center;
  user-select: none;
}

.thumbnail-strip {
  flex-shrink: 0;
  background: #16213e;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding: 8px 0;
  overflow-x: auto;
  overflow-y: hidden;
}

.thumbnail-scroll {
  display: flex;
  gap: 8px;
  padding: 0 16px;
  width: max-content;
}

.thumbnail-item {
  flex-shrink: 0;
  cursor: pointer;
  border: 2px solid transparent;
  border-radius: 4px;
  overflow: hidden;
  transition: border-color 0.2s, transform 0.2s;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 2px;
}

.thumbnail-item:hover {
  border-color: #666;
  transform: translateY(-2px);
}

.thumbnail-item.active {
  border-color: #409eff;
}

.thumbnail-image {
  width: 80px;
  height: 45px;
  object-fit: cover;
  display: block;
  border-radius: 2px;
}

.thumbnail-num {
  color: #aaa;
  font-size: 10px;
}

.ppt-loading {
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

.ppt-error,
.ppt-empty {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
}

@media screen and (max-width: 767px) {
  .ppt-main {
    padding: 8px;
  }

  .ppt-footer {
    padding: 8px;
  }

  .slide-nav .el-button {
    width: 40px;
    height: 40px;
  }

  .slide-counter {
    font-size: 16px;
  }

  .thumbnail-strip {
    padding: 6px 0;
  }

  .thumbnail-scroll {
    padding: 0 8px;
    gap: 6px;
  }

  .thumbnail-image {
    width: 56px;
    height: 32px;
  }

  .thumbnail-num {
    font-size: 9px;
  }
}
</style>
