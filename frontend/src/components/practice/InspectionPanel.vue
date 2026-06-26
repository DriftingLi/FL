<template>
  <div class="practice-panel">
    <div class="panel-header">
      <h4>📋 日常检查</h4>
      <el-tag :type="completed ? 'success' : 'info'" size="small">
        {{ completed ? '已完成' : `步骤 ${step + 1}/${steps.length}` }}
      </el-tag>
    </div>
    <div v-if="!completed" class="panel-body">
      <div class="step-list">
        <div v-for="(s, index) in steps" :key="s.partId"
          class="step-item" :class="{
            'step-active': index === step,
            'step-done': index < step,
            'step-pending': index > step
          }">
          <span class="step-icon">
            <el-icon v-if="index < step"><CircleCheckFilled /></el-icon>
            <el-icon v-else-if="index === step"><Right /></el-icon>
            <span v-else>{{ index + 1 }}</span>
          </span>
          <span class="step-name">{{ s.name }}</span>
        </div>
      </div>
      <div class="step-hint" v-if="step < steps.length">
        <el-alert type="info" :closable="false" show-icon>
          <template #title>请点击叉车上的 <strong>{{ currentPart?.name }}</strong> 完成检查</template>
        </el-alert>
        <div class="step-actions">
          <el-button v-if="step > 0" size="small" @click="$emit('undo')">撤销上一步</el-button>
        </div>
      </div>
    </div>
    <div v-else class="panel-body">
      <el-result icon="success" title="日常检查完成！" :sub-title="`用时: ${duration}秒`">
        <template #extra>
          <el-button type="primary" size="small" @click="$emit('restart')">重新开始</el-button>
        </template>
      </el-result>
    </div>
  </div>
</template>

<script setup lang="ts">
import { CircleCheckFilled, Right } from '@element-plus/icons-vue'

defineProps({
  step: { type: Number, required: true },
  steps: { type: Array, required: true },
  currentPart: { type: Object, default: null },
  completed: { type: Boolean, required: true },
  duration: { type: Number, default: 0 }
})

defineEmits(['restart', 'undo'])
</script>

<style scoped>
.practice-panel {
  position: absolute;
  top: 70px;
  left: 12px;
  width: 260px;
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

.step-list {
  margin-bottom: 12px;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  font-size: 13px;
  transition: all 0.2s;
}

.step-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 12px;
  flex-shrink: 0;
}

.step-active .step-icon {
  color: #409eff;
}

.step-active .step-name {
  color: #409eff;
  font-weight: 600;
}

.step-done .step-icon {
  color: #67c23a;
}

.step-done .step-name {
  color: #67c23a;
  text-decoration: line-through;
}

.step-pending .step-icon {
  color: #c0c4cc;
  border: 1px solid #c0c4cc;
  border-radius: 50%;
}

.step-pending .step-name {
  color: #909399;
}

.step-hint {
  margin-top: 8px;
}

.step-actions {
  margin-top: 8px;
  text-align: right;
}

@media screen and (max-width: 768px) {
  .practice-panel {
    top: auto;
    bottom: 50px;
    left: 8px;
    right: 8px;
    width: auto;
    max-height: 40%;
  }
}
</style>
