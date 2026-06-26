<template>
  <div v-if="visible" class="guided-tour-overlay" @click.self="skip">
    <div class="tour-card" :style="cardStyle">
      <div class="tour-step-indicator">
        <span v-for="i in steps.length" :key="i"
          class="step-dot" :class="{ active: i - 1 === currentStep, done: i - 1 < currentStep }" />
      </div>
      <h3 class="tour-title">{{ currentStepData.title }}</h3>
      <p class="tour-desc">{{ currentStepData.desc }}</p>
      <div class="tour-actions">
        <el-button size="small" @click="skip">跳过教程</el-button>
        <el-button size="small" @click="prev" :disabled="currentStep === 0">上一步</el-button>
        <el-button size="small" type="primary" @click="next">
          {{ currentStep === steps.length - 1 ? '开始实操' : '下一步' }}
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps({
  visible: { type: Boolean, default: false },
  currentStep: { type: Number, default: 0 }
})

const emit = defineEmits(['next', 'prev', 'skip'])

const steps = [
  { title: '欢迎来到虚拟实操', desc: '这里你可以通过3D叉车模型进行维修培训实操。让我们快速了解一下操作方式。', position: 'center' },
  { title: '旋转视角', desc: '按住鼠标左键并拖动，可以旋转观察叉车的不同角度。', position: 'center' },
  { title: '平移视角', desc: '按住鼠标右键并拖动，可以平移视角查看叉车的不同部位。', position: 'center' },
  { title: '缩放视角', desc: '滚动鼠标滚轮，可以放大或缩小视角，近距离观察部件细节。', position: 'center' },
  { title: '点击查看部件', desc: '将鼠标悬停在叉车部件上会显示部件名称，点击部件可查看详细信息和维护要点。', position: 'center' },
  { title: '选择实操模式', desc: '顶部工具栏可切换三种模式：自由探索（随意查看）、日常检查（按步骤检查）、故障诊断（找出故障部件）。', position: 'top' },
  { title: '准备就绪！', desc: '现在你可以开始实操了。建议从"自由探索"模式开始，熟悉叉车各部件后再尝试其他模式。', position: 'center' }
]

const currentStepData = computed(() => steps[props.currentStep] || steps[0])

const cardStyle = computed(() => {
  const pos = currentStepData.value.position
  if (pos === 'top') {
    return { top: '80px', left: '50%', transform: 'translateX(-50%)' }
  }
  return { top: '50%', left: '50%', transform: 'translate(-50%, -50%)' }
})

function next() {
  if (props.currentStep >= steps.length - 1) {
    emit('skip')
  } else {
    emit('next')
  }
}

function prev() {
  emit('prev')
}

function skip() {
  emit('skip')
}
</script>

<style scoped>
.guided-tour-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
}

.tour-card {
  position: absolute;
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  width: 340px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  text-align: center;
}

.tour-step-indicator {
  display: flex;
  justify-content: center;
  gap: 6px;
  margin-bottom: 16px;
}

.step-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #dcdfe6;
  transition: all 0.2s;
}

.step-dot.active {
  background: #409eff;
  transform: scale(1.3);
}

.step-dot.done {
  background: #67c23a;
}

.tour-title {
  font-size: 18px;
  color: #303133;
  margin: 0 0 10px;
}

.tour-desc {
  font-size: 14px;
  color: #606266;
  line-height: 1.6;
  margin: 0 0 20px;
}

.tour-actions {
  display: flex;
  justify-content: center;
  gap: 8px;
}

@media screen and (max-width: 480px) {
  .tour-card {
    width: calc(100% - 32px);
    padding: 16px;
  }
}
</style>
