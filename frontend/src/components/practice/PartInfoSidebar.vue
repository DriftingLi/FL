<template>
  <transition name="slide-right">
    <div v-if="part" class="part-info-sidebar">
      <div class="sidebar-header">
        <h3>{{ part.name }}</h3>
        <el-button text @click="$emit('close')" :icon="Close" />
      </div>
      <div class="sidebar-content">
        <p class="part-desc">{{ part.info }}</p>
        <div v-if="part.maintenance && part.maintenance.length" class="maintenance-tips">
          <h4>维护要点</h4>
          <ul>
            <li v-for="tip in part.maintenance" :key="tip">{{ tip }}</li>
          </ul>
        </div>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { Close } from '@element-plus/icons-vue'

defineProps({
  part: { type: Object, default: null }
})

defineEmits(['close'])
</script>

<style scoped>
.part-info-sidebar {
  position: absolute;
  top: 70px;
  right: 12px;
  width: 280px;
  max-height: calc(100% - 160px);
  background: rgba(255, 255, 255, 0.95);
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.15);
  z-index: 10;
  overflow-y: auto;
}

.sidebar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid #ebeef5;
}

.sidebar-header h3 {
  font-size: 16px;
  color: #303133;
  margin: 0;
}

.sidebar-content {
  padding: 12px 16px;
}

.part-desc {
  color: #606266;
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 12px;
}

.maintenance-tips h4 {
  font-size: 14px;
  color: #303133;
  margin-bottom: 8px;
}

.maintenance-tips ul {
  padding-left: 16px;
  margin: 0;
}

.maintenance-tips li {
  color: #606266;
  font-size: 13px;
  line-height: 1.8;
}

.slide-right-enter-active,
.slide-right-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease;
}

.slide-right-enter-from {
  transform: translateX(20px);
  opacity: 0;
}

.slide-right-leave-to {
  transform: translateX(20px);
  opacity: 0;
}

@media screen and (max-width: 768px) {
  .part-info-sidebar {
    top: auto;
    bottom: 50px;
    right: 8px;
    left: 8px;
    width: auto;
    max-height: 40%;
  }
}
</style>
