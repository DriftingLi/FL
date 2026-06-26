<template>
  <div class="demo-panel">
    <div class="panel-header">
      <h4>🎬 动画演示</h4>
      <el-button v-if="!isPlaying" size="small" text @click="$emit('close')">✕</el-button>
    </div>

    <div v-if="!currentAnimation" class="panel-body">
      <div class="animation-list">
        <div v-for="anim in animations" :key="anim.id" class="animation-item" @click="$emit('select', anim.id)">
          <div class="anim-info">
            <span class="anim-name">{{ anim.name }}</span>
            <span class="anim-duration">{{ anim.duration }}秒</span>
          </div>
          <p class="anim-desc">{{ anim.description }}</p>
        </div>
      </div>
    </div>

    <div v-else class="panel-body">
      <div class="anim-title">{{ currentAnimation.name }}</div>

      <div class="narration-box" v-if="narration">
        <el-alert type="info" :closable="false" show-icon>
          <template #title>{{ narration }}</template>
        </el-alert>
      </div>

      <div class="progress-bar">
        <el-slider
          :model-value="progress"
          :min="0"
          :max="100"
          :step="0.1"
          size="small"
          @input="$emit('seek', $event / 100)"
        />
        <span class="time-display">{{ formatTime(currentTime) }} / {{ formatTime(duration) }}</span>
      </div>

      <div class="playback-controls">
        <el-button-group>
          <el-button size="small" @click="$emit('stop')" :icon="VideoPause">停止</el-button>
          <el-button size="small" :type="isPlaying ? 'warning' : 'primary'" @click="$emit(isPlaying ? 'pause' : 'play')">
            {{ isPlaying ? '暂停' : '播放' }}
          </el-button>
        </el-button-group>

        <el-select size="small" :model-value="speed" style="width: 80px;" @change="$emit('speed', $event)">
          <el-option label="0.5x" :value="0.5" />
          <el-option label="1x" :value="1" />
          <el-option label="1.5x" :value="1.5" />
          <el-option label="2x" :value="2" />
        </el-select>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { VideoPause } from '@element-plus/icons-vue'

defineProps({
  animations: { type: Array, default: () => [] },
  currentAnimation: { type: Object, default: null },
  isPlaying: { type: Boolean, default: false },
  progress: { type: Number, default: 0 },
  currentTime: { type: Number, default: 0 },
  duration: { type: Number, default: 0 },
  speed: { type: Number, default: 1 },
  narration: { type: String, default: '' }
})

defineEmits(['select', 'play', 'pause', 'stop', 'seek', 'speed', 'close'])

function formatTime(seconds) {
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}
</script>

<style scoped>
.demo-panel {
  position: absolute;
  top: 70px;
  left: 12px;
  width: 280px;
  max-height: calc(100% - 160px);
  background: rgba(255, 255, 255, 0.95);
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
  z-index: 10;
  overflow-y: auto;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #ebeef5;
}

.panel-header h4 {
  font-size: 15px;
  color: #303133;
  margin: 0;
}

.panel-body {
  padding: 12px 16px;
}

.animation-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.animation-item {
  padding: 10px;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.animation-item:hover {
  border-color: #409eff;
  background: #ecf5ff;
}

.anim-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.anim-name {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.anim-duration {
  font-size: 12px;
  color: #909399;
}

.anim-desc {
  font-size: 12px;
  color: #606266;
  margin: 0;
  line-height: 1.4;
}

.anim-title {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 10px;
}

.narration-box {
  margin-bottom: 10px;
}

.progress-bar {
  margin-bottom: 10px;
}

.time-display {
  font-size: 12px;
  color: #909399;
  display: block;
  text-align: center;
  margin-top: 4px;
  font-variant-numeric: tabular-nums;
}

.playback-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
