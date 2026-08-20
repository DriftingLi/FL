<template>
  <div class="video-player-wrapper" ref="wrapperRef">
    <div v-if="error" class="video-error">
      <el-empty :description="errorMessage">
        <el-button type="primary" @click="retryLoad">重试</el-button>
        <el-button @click="downloadVideo">下载视频</el-button>
      </el-empty>
    </div>

    <div v-else class="video-container">
      <video
        ref="videoRef"
        class="video-player"
        controls
        preload="metadata"
        :src="resolvedSrc"
        @error="handleError"
        @loadedmetadata="onLoadedMetadata"
        @waiting="onWaiting"
        @canplay="onCanPlay"
        @timeupdate="onTimeUpdate"
        @pause="onPause"
        @ended="onEnded"
      >
        您的浏览器不支持视频播放
      </video>

      <div v-if="buffering" class="video-buffering">
        <el-icon class="buffering-icon" :size="36"><Loading /></el-icon>
      </div>

      <div class="video-controls-overlay">
        <el-dropdown trigger="click" @command="changeSpeed">
          <el-button size="small" class="control-btn">
            {{ currentSpeed }}x
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="speed in speedOptions"
                :key="speed"
                :command="speed"
                :class="{ 'is-active': currentSpeed === speed }"
              >
                {{ speed }}x
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <el-tooltip content="画中画" placement="top">
          <el-button
            size="small"
            class="control-btn"
            @click="togglePiP"
            :disabled="!pipSupported"
          >
            <el-icon><Monitor /></el-icon>
          </el-button>
        </el-tooltip>

        <el-tooltip content="下载" placement="top">
          <el-button size="small" class="control-btn" @click="downloadVideo">
            <el-icon><Download /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { Download, Loading, Monitor } from '@element-plus/icons-vue'
import { resolveFileUrl } from '@/utils/fileUrl'

const props = defineProps({
  src: { type: String, required: true },
  /** 断点续播位置（秒，ADR-0017）：loadedmetadata 时自动 seek */
  initialPosition: { type: Number, default: 0 }
})

// position-update：节流上报当前播放位置（秒），父组件随进度上报落库
const emit = defineEmits<{
  (e: 'position-update', seconds: number): void
}>()

const resolvedSrc = computed(() => resolveFileUrl(props.src))

const videoRef = ref<HTMLVideoElement | null>(null)
const wrapperRef = ref<HTMLDivElement | null>(null)
const error = ref(false)
const errorMessage = ref('视频加载失败，请稍后重试')
const buffering = ref(false)
const currentSpeed = ref(1)

const speedOptions = [0.5, 0.75, 1, 1.25, 1.5, 2]

const pipSupported = computed(() => {
  return document.pictureInPictureEnabled
})

function handleError() {
  error.value = true
  checkFileExists()
}

async function checkFileExists() {
  try {
    const response = await fetch(resolvedSrc.value, { method: 'HEAD' })
    if (response.status === 404) {
      errorMessage.value = '视频文件不存在或已过期，请重新上传'
    } else if (!response.ok) {
      errorMessage.value = `视频加载失败 (${response.status})`
    }
  } catch (e) {
    errorMessage.value = '无法连接到文件服务器'
  }
}

function retryLoad() {
  error.value = false
  if (videoRef.value) {
    videoRef.value.load()
  }
}

function onLoadedMetadata() {
  error.value = false
  // 断点续播：恢复上次播放位置（保留 5 秒回退缓冲，便于衔接上下文）
  if (props.initialPosition > 0 && videoRef.value) {
    const target = Math.max(props.initialPosition - 5, 0)
    if (target < videoRef.value.duration) {
      videoRef.value.currentTime = target
    }
  }
}

// 播放位置节流上报（10s 一次；pause/ended 立即上报），父组件随学习进度上报落库
const POSITION_EMIT_INTERVAL = 10
let lastEmittedPosition = 0
let lastEmitTime = 0

function emitPosition(force = false) {
  const video = videoRef.value
  if (!video || !isFinite(video.currentTime)) return
  const now = Date.now()
  if (!force && now - lastEmitTime < POSITION_EMIT_INTERVAL * 1000) return
  lastEmitTime = now
  lastEmittedPosition = Math.floor(video.currentTime)
  emit('position-update', lastEmittedPosition)
}

function onTimeUpdate() {
  emitPosition()
}

function onPause() {
  emitPosition(true)
}

function onEnded() {
  emitPosition(true)
}

function onWaiting() {
  buffering.value = true
}

function onCanPlay() {
  buffering.value = false
}

function changeSpeed(speed: number) {
  currentSpeed.value = speed
  if (videoRef.value) {
    videoRef.value.playbackRate = speed
  }
}

async function togglePiP() {
  if (!videoRef.value) return
  try {
    if (document.pictureInPictureElement) {
      await document.exitPictureInPicture()
    } else {
      await videoRef.value.requestPictureInPicture()
    }
  } catch (e) {
    console.error('PiP error:', e)
  }
}

function downloadVideo() {
  const link = document.createElement('a')
  link.href = resolvedSrc.value
  link.download = ''
  link.click()
}

onBeforeUnmount(() => {
  emitPosition(true)
  if (document.pictureInPictureElement) {
    document.exitPictureInPicture()
  }
})
</script>

<style scoped>
.video-player-wrapper {
  width: 100%;
  background: #000;
  border-radius: 8px;
  overflow: hidden;
  position: relative;
}

.video-container {
  position: relative;
}

.video-player {
  width: 100%;
  max-height: 500px;
  display: block;
}

.video-buffering {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 5;
  pointer-events: none;
}

.buffering-icon {
  color: rgba(255, 255, 255, 0.8);
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.video-controls-overlay {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  gap: 4px;
  z-index: 6;
  opacity: 0;
  transition: opacity 0.3s ease;
}

.video-container:hover .video-controls-overlay {
  opacity: 1;
}

.control-btn {
  --el-button-bg-color: rgba(0, 0, 0, 0.6);
  --el-button-border-color: transparent;
  --el-button-text-color: #fff;
  --el-button-hover-bg-color: rgba(0, 0, 0, 0.8);
  --el-button-hover-border-color: transparent;
  --el-button-hover-text-color: #fff;
  padding: 4px 8px;
  font-size: 12px;
}

.video-error {
  padding: 40px 20px;
  background: #fff;
}
</style>
