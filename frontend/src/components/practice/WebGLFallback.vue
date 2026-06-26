<template>
  <div v-if="visible" class="webgl-fallback">
    <el-icon :size="64" color="#E6A23C"><WarningFilled /></el-icon>
    <h3>您的浏览器不支持WebGL</h3>
    <p>虚拟实操功能需要WebGL支持，请使用Chrome、Firefox或Edge最新版本浏览器</p>
    <div class="static-preview">
      <div class="preview-card">
        <div class="preview-icon">🔧</div>
        <h4>叉车结构概览</h4>
        <div class="parts-list">
          <div v-for="part in partsConfig" :key="part.partId" class="part-item">
            <strong>{{ part.name }}</strong>
            <p>{{ part.info }}</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { WarningFilled } from '@element-plus/icons-vue'
import { getPartsConfig } from '@/utils/forkliftModel'

defineProps({
  visible: { type: Boolean, default: false }
})

const partsConfig = getPartsConfig()
</script>

<style scoped>
.webgl-fallback {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
}

.webgl-fallback h3 {
  font-size: 20px;
  color: #303133;
  margin: 20px 0 10px;
}

.webgl-fallback p {
  color: #909399;
  font-size: 14px;
  margin-bottom: 30px;
}

.static-preview {
  max-width: 600px;
  width: 100%;
}

.preview-card {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.preview-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

.preview-card h4 {
  font-size: 18px;
  color: #303133;
  margin-bottom: 16px;
}

.parts-list {
  text-align: left;
}

.part-item {
  padding: 10px 0;
  border-bottom: 1px solid #ebeef5;
}

.part-item:last-child {
  border-bottom: none;
}

.part-item strong {
  color: #409eff;
  font-size: 14px;
}

.part-item p {
  color: #606266;
  font-size: 13px;
  margin-top: 4px;
  line-height: 1.5;
}
</style>
