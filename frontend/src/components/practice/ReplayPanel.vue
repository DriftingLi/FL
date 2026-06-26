<template>
  <div class="replay-panel">
    <div class="panel-header">
      <h4>🔄 操作回放</h4>
      <el-button size="small" text @click="$emit('close')">✕</el-button>
    </div>

    <div v-if="!replaying" class="panel-body">
      <div v-if="operations.length === 0" class="empty-hint">
        <el-empty description="暂无操作记录" :image-size="60" />
      </div>
      <div v-else class="operation-list">
        <div v-for="(op, index) in operations" :key="index"
          class="op-item" :class="{ 'op-current': index === currentStep }">
          <span class="op-index">{{ index + 1 }}</span>
          <span class="op-part">{{ op.partName || op.partId }}</span>
          <el-tag size="small" :type="getActionType(op.action)">{{ getActionLabel(op.action) }}</el-tag>
        </div>
      </div>
      <div class="replay-actions" v-if="operations.length > 0">
        <el-button type="primary" size="small" @click="$emit('start-replay')">开始回放</el-button>
      </div>
    </div>

    <div v-else class="panel-body">
      <div class="replay-status">
        <el-tag type="info" size="small">步骤 {{ currentStep + 1 }} / {{ operations.length }}</el-tag>
      </div>

      <div v-if="currentOp" class="current-op-info">
        <el-alert type="info" :closable="false" show-icon>
          <template #title>
            {{ currentOp.partName || currentOp.partId }} - {{ getActionLabel(currentOp.action) }}
          </template>
        </el-alert>
      </div>

      <div class="progress-bar">
        <el-slider
          :model-value="replayProgress * 100"
          :min="0"
          :max="100"
          :step="1"
          size="small"
          @input="$emit('seek', Math.round($event / 100 * operations.length))"
        />
      </div>

      <div class="playback-controls">
        <el-button-group>
          <el-button size="small" @click="$emit('stop-replay')">停止</el-button>
          <el-button size="small" :type="isPlaying ? 'warning' : 'primary'" @click="$emit(isPlaying ? 'pause-replay' : 'resume-replay')">
            {{ isPlaying ? '暂停' : '继续' }}
          </el-button>
        </el-button-group>
        <el-select size="small" :model-value="speed" style="width: 80px;" @change="$emit('speed', $event)">
          <el-option label="0.5x" :value="0.5" />
          <el-option label="1x" :value="1" />
          <el-option label="2x" :value="2" />
          <el-option label="3x" :value="3" />
        </el-select>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps({
  operations: { type: Array, default: () => [] },
  currentStep: { type: Number, default: 0 },
  currentOp: { type: Object, default: null },
  replaying: { type: Boolean, default: false },
  isPlaying: { type: Boolean, default: false },
  replayProgress: { type: Number, default: 0 },
  speed: { type: Number, default: 1 }
})

defineEmits(['close', 'start-replay', 'stop-replay', 'pause-replay', 'resume-replay', 'seek', 'speed'])

function getActionType(action) {
  const map = { click: 'info', detach: 'warning', attach: 'success', undo: '' }
  return map[action] || 'info'
}

function getActionLabel(action) {
  const map = { click: '点击', detach: '拆卸', attach: '装回', undo: '撤销' }
  return map[action] || action
}
</script>

<style scoped>
.replay-panel {
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

.operation-list {
  max-height: 300px;
  overflow-y: auto;
  margin-bottom: 12px;
}

.op-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 4px;
  font-size: 13px;
  transition: background 0.2s;
}

.op-item:hover {
  background: #f5f7fa;
}

.op-item.op-current {
  background: #ecf5ff;
  border: 1px solid #b3d8ff;
}

.op-index {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: #e4e7ed;
  color: #606266;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  flex-shrink: 0;
}

.op-current .op-index {
  background: #409eff;
  color: #fff;
}

.op-part {
  flex: 1;
  color: #303133;
}

.replay-actions {
  text-align: center;
  padding-top: 8px;
  border-top: 1px solid #ebeef5;
}

.replay-status {
  margin-bottom: 10px;
}

.current-op-info {
  margin-bottom: 10px;
}

.progress-bar {
  margin-bottom: 10px;
}

.playback-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.empty-hint {
  padding: 20px 0;
}

@media screen and (max-width: 768px) {
  .replay-panel {
    top: auto;
    bottom: 50px;
    left: 8px;
    right: 8px;
    width: auto;
    max-height: 40%;
  }
}
</style>
