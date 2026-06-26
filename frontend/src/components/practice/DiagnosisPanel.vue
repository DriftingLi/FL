<template>
  <div class="practice-panel">
    <div class="panel-header">
      <h4>🔍 故障诊断</h4>
      <el-tag v-if="completed" type="success" size="small">已完成</el-tag>
    </div>
    <div v-if="!completed" class="panel-body">
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>故障现象：{{ hint }}</template>
      </el-alert>
      <div class="diagnosis-info">
        <p>请点击您认为出现故障的部件</p>
        <p>尝试次数: <strong>{{ attempts }}</strong></p>
      </div>
    </div>
    <div v-else class="panel-body">
      <el-result
        :icon="attempts <= 2 ? 'success' : 'warning'"
        :title="attempts === 1 ? '诊断准确！' : '诊断完成'"
        :sub-title="`得分: ${score}分 | 用时: ${duration}秒 | 故障: ${fault}`">
        <template #extra>
          <el-button type="primary" size="small" @click="$emit('restart')">重新开始</el-button>
        </template>
      </el-result>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps({
  hint: { type: String, default: '' },
  attempts: { type: Number, default: 0 },
  completed: { type: Boolean, required: true },
  score: { type: Number, default: 0 },
  duration: { type: Number, default: 0 },
  fault: { type: String, default: '' }
})

defineEmits(['restart'])
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

.diagnosis-info {
  margin-top: 12px;
}

.diagnosis-info p {
  font-size: 13px;
  color: #606266;
  margin: 6px 0;
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
