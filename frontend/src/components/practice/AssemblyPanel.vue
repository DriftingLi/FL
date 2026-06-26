<template>
  <div class="assembly-panel">
    <div class="panel-header">
      <h4>🔧 部件拆装</h4>
      <el-button size="small" text @click="$emit('reset')">重置</el-button>
    </div>
    <div class="panel-body">
      <div class="part-list">
        <div v-for="part in assemblyOrder" :key="part.partId" class="assembly-item">
          <span class="part-name">{{ part.name }}</span>
          <div class="part-actions">
            <el-tag v-if="partStates[part.partId] === 'attached'" type="success" size="small">已安装</el-tag>
            <el-tag v-else-if="partStates[part.partId] === 'detached'" type="warning" size="small">已拆卸</el-tag>
            <el-tag v-else type="info" size="small">移动中</el-tag>

            <el-button
              v-if="partStates[part.partId] === 'attached'"
              size="small"
              type="danger"
              plain
              :disabled="!canDetachPart(part.partId)"
              @click="$emit('detach', part.partId)"
            >拆卸</el-button>

            <el-button
              v-if="partStates[part.partId] === 'detached'"
              size="small"
              type="success"
              plain
              :disabled="!canAttachPart(part.partId)"
              @click="$emit('attach', part.partId)"
            >装回</el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { getPartsConfig } from '@/utils/forkliftModel'
import { getDisassemblyOrder, canDetach, canAttach } from '@/utils/partAssembly'

const props = defineProps({
  partStates: { type: Object, required: true }
})

defineEmits(['detach', 'attach', 'reset'])

const partsConfig = getPartsConfig()
const disassemblyOrder = getDisassemblyOrder()

const assemblyOrder = disassemblyOrder.map(partId => {
  const config = partsConfig.find(p => p.partId === partId)
  return config || { partId, name: partId }
})

function canDetachPart(partId) {
  return canDetach(partId, props.partStates)
}

function canAttachPart(partId) {
  return canAttach(partId, props.partStates)
}
</script>

<style scoped>
.assembly-panel {
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

.part-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.assembly-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  font-size: 13px;
}

.part-name {
  color: #303133;
  font-weight: 500;
}

.part-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

@media screen and (max-width: 768px) {
  .assembly-panel {
    top: auto;
    bottom: 50px;
    left: 8px;
    right: 8px;
    width: auto;
    max-height: 40%;
  }
}
</style>
