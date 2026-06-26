<template>
  <div class="toolbar">
    <div class="toolbar-left">
      <el-radio-group :model-value="practiceMode" size="small" @change="$emit('update:practiceMode', $event)">
        <el-radio-button value="free">自由探索</el-radio-button>
        <el-radio-button value="inspection">日常检查</el-radio-button>
        <el-radio-button value="diagnosis">故障诊断</el-radio-button>
        <el-radio-button value="assembly">部件拆装</el-radio-button>
        <el-radio-button value="demo">动画演示</el-radio-button>
      </el-radio-group>
      <el-select
        v-if="practiceMode === 'inspection' || practiceMode === 'diagnosis'"
        :model-value="difficulty"
        size="small"
        style="width: 90px; margin-left: 8px;"
        @change="$emit('update:difficulty', $event)"
      >
        <el-option label="初级" value="beginner" />
        <el-option label="中级" value="normal" />
        <el-option label="高级" value="expert" />
      </el-select>
    </div>
    <div class="toolbar-right">
      <el-button size="small" @click="$emit('show-replay')" :icon="VideoPlay">回放</el-button>
      <el-button size="small" @click="$emit('show-report')" :icon="DataAnalysis">报告</el-button>
      <el-switch
        :model-value="soundEnabled"
        active-text="音效"
        inactive-text=""
        size="small"
        @update:model-value="$emit('update:soundEnabled', $event)"
      />
      <el-switch :model-value="lowQualityMode" active-text="低画质" inactive-text="" size="small"
        @update:model-value="$emit('update:lowQualityMode', $event)" />
      <el-button size="small" @click="$emit('takeScreenshot')" :icon="Camera">截图</el-button>
      <el-button size="small" @click="$emit('resetCamera')" :icon="RefreshRight">重置视角</el-button>
      <el-button size="small" @click="$emit('toggleFullscreen')" :icon="FullScreen">全屏</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { RefreshRight, FullScreen, Camera, VideoPlay, DataAnalysis } from '@element-plus/icons-vue'

defineProps({
  practiceMode: { type: String, required: true },
  lowQualityMode: { type: Boolean, required: true },
  difficulty: { type: String, default: 'normal' },
  soundEnabled: { type: Boolean, default: true }
})

defineEmits(['update:practiceMode', 'update:lowQualityMode', 'update:difficulty', 'update:soundEnabled', 'resetCamera', 'toggleFullscreen', 'takeScreenshot', 'show-replay', 'show-report'])
</script>

<style scoped>
.toolbar {
  position: absolute;
  top: 12px;
  left: 12px;
  right: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  background: rgba(255, 255, 255, 0.95);
  border-radius: 8px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  z-index: 10;
}

.toolbar-left {
  display: flex;
  align-items: center;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media screen and (max-width: 768px) {
  .toolbar {
    top: 8px;
    left: 8px;
    right: 8px;
    padding: 6px 10px;
    flex-wrap: wrap;
    gap: 8px;
  }

  .toolbar-left {
    width: 100%;
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }

  .toolbar-left :deep(.el-radio-group) {
    flex-wrap: nowrap;
  }

  .toolbar-right {
    width: 100%;
    justify-content: flex-end;
  }
}

@media screen and (max-width: 480px) {
  .toolbar-right .el-switch {
    display: none;
  }
}
</style>
